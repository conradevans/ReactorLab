package observability

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/system"
)

const (
	HostInterval              = 5 * time.Second
	TemperatureInterval       = time.Second
	TemperatureBucketInterval = 5 * time.Second
	ApplicationInterval       = 10 * time.Second
	ServiceInterval           = 10 * time.Second
	EventInterval             = 30 * time.Second
	RetentionInterval         = time.Hour
	metricQueueSize           = 64
	criticalQueueSize         = 16
	criticalWriteAttempts     = 3
	criticalRetryDelay        = 50 * time.Millisecond
	writerOperationTimeout    = 10 * time.Second
	writerShutdownTimeout     = 5 * time.Second
	finalMetricEnqueueTimeout = 2 * time.Second
)

type deploymentSource interface {
	Deployments(context.Context) (minideploy.Snapshot, error)
}

type miniBaseActivitySource interface {
	Activity(context.Context) ([]minibase.Activity, error)
}

type collectorStore interface {
	InsertBatch(context.Context, Batch) error
	PruneMetrics(context.Context, time.Time) error
	State(context.Context, string) (string, bool, error)
}

type writerMessage struct {
	batch       Batch
	pruneBefore *time.Time
}

type CollectorConfig struct {
	MiniDeployURL          string
	MiniBaseURL            string
	MiniAIURL              string
	RecoveryDiscordEnabled bool
}

type Collector struct {
	store                  collectorStore
	deployments            deploymentSource
	activities             miniBaseActivitySource
	health                 *healthChecker
	readHost               func(time.Time) (system.HistoricalSnapshot, error)
	readTemperature        func() (system.TemperatureStats, bool)
	now                    func() time.Time
	hostTracker            HostTracker
	sessionTracker         HostSessionTracker
	appTracker             ApplicationTracker
	serviceTracker         ServiceTracker
	metricQueue            chan writerMessage
	criticalQueue          chan writerMessage
	retryDelay             time.Duration
	errors                 *errorReporter
	intervals              collectorIntervals
	recoveryDiscordEnabled bool
}

type collectorIntervals struct {
	host, temperature, temperatureBucket, applications, services, events, retention time.Duration
}

func NewCollector(store *Store, config CollectorConfig) *Collector {
	miniDeployURL := normalizedURL(config.MiniDeployURL, minideploy.DefaultBaseURL)
	miniBaseURL := normalizedURL(config.MiniBaseURL, minibase.DefaultBaseURL)
	miniAIURL := normalizedURL(config.MiniAIURL, "http://127.0.0.1:9300")
	return &Collector{
		store:           store,
		deployments:     minideploy.NewClient(miniDeployURL, 4*time.Second),
		activities:      minibase.NewClient(miniBaseURL, 4*time.Second),
		health:          newHealthChecker(miniDeployURL, miniBaseURL, miniAIURL),
		readHost:        system.ReadHistoricalSnapshot,
		readTemperature: system.ReadTemperature,
		now:             time.Now,
		metricQueue:     make(chan writerMessage, metricQueueSize),
		criticalQueue:   make(chan writerMessage, criticalQueueSize),
		retryDelay:      criticalRetryDelay,
		errors:          newErrorReporter(time.Minute),
		intervals: collectorIntervals{
			host: HostInterval, temperature: TemperatureInterval,
			temperatureBucket: TemperatureBucketInterval, applications: ApplicationInterval,
			services: ServiceInterval, events: EventInterval, retention: RetentionInterval,
		},
		recoveryDiscordEnabled: config.RecoveryDiscordEnabled,
	}
}

func (c *Collector) Run(ctx context.Context) {
	if c == nil || c.store == nil {
		return
	}
	c.loadBaselines(ctx)
	var producers sync.WaitGroup
	writerDone := make(chan struct{})
	writerContext, cancelWriter := context.WithCancel(context.Background())
	defer cancelWriter()
	go func() {
		defer close(writerDone)
		c.writeLoop(writerContext)
	}()
	start := func(run func(context.Context)) {
		producers.Add(1)
		go func() { defer producers.Done(); run(ctx) }()
	}
	start(c.hostLoop)
	start(c.temperatureLoop)
	start(c.applicationLoop)
	start(c.serviceLoop)
	start(c.eventLoop)
	start(c.retentionLoop)
	<-ctx.Done()
	producers.Wait()
	close(c.metricQueue)
	close(c.criticalQueue)
	shutdownTimer := time.AfterFunc(writerShutdownTimeout, cancelWriter)
	<-writerDone
	shutdownTimer.Stop()
	c.persistCleanShutdown()
}

func (c *Collector) loadBaselines(ctx context.Context) {
	if bootID, ok, err := c.store.State(ctx, hostBootStateKey); err == nil && ok {
		c.hostTracker.SetBootBaseline(bootID)
	}
	if value, ok, err := c.store.State(ctx, hostSessionStateKey); err != nil {
		c.errors.report("host_session", "load host session failed", err)
	} else if ok {
		if err := c.sessionTracker.SetBaseline(value); err != nil {
			c.errors.report("host_session", "invalid persisted host session ignored", err)
		}
	}
	for _, service := range c.health.services {
		if value, ok, err := c.store.State(ctx, serviceStatePrefix+service.id); err == nil && ok {
			if available, parseErr := strconv.ParseBool(value); parseErr == nil {
				c.serviceTracker.SetHealthBaseline(service.id, available)
			}
		}
		if value, ok, err := c.store.State(ctx, serviceInvocationPrefix+service.id); err == nil && ok {
			c.serviceTracker.SetInvocationBaseline(service.id, value)
		}
	}
}

func (c *Collector) hostLoop(ctx context.Context) {
	runPeriodic(ctx, c.intervals.host, true, func() {
		now := c.now().UTC()
		snapshot, err := c.readHost(now)
		if err != nil {
			c.errors.report("host", "historical host collection failed", err)
			return
		}
		sample, events, state := c.hostTracker.Observe(snapshot)
		session, sessionErr := c.sessionTracker.Observe(
			snapshot.BootID,
			snapshot.ObservedAt,
			snapshot.UptimeSeconds,
		)
		if sessionErr != nil {
			c.errors.report("host_session", "host recovery session observation was incomplete", sessionErr)
		}
		if session.StateValue != "" {
			state[hostSessionStateKey] = session.StateValue
		}
		var recoveries []RecoveryIncidentWrite
		if session.Event != nil {
			recoveries = append(recoveries, RecoveryIncidentWrite{
				Event:  *session.Event,
				Notify: c.recoveryDiscordEnabled,
			})
		}
		message := writerMessage{batch: Batch{
			Hosts:      []HostSample{sample},
			Events:     events,
			Recoveries: recoveries,
			State:      state,
		}}
		if session.Critical || len(events) > 0 || len(recoveries) > 0 {
			c.enqueueCritical(ctx, message)
		} else {
			c.enqueueMetric(message)
		}
	})
}
func (c *Collector) persistCleanShutdown() {
	value, ok, err := c.sessionTracker.MarkClean()
	if err != nil {
		c.errors.report("host_session_shutdown", "encode clean host session failed", err)
		return
	}
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), writerOperationTimeout)
	defer cancel()
	if err := c.store.InsertBatch(ctx, Batch{
		State: map[string]string{hostSessionStateKey: value},
	}); err != nil {
		c.errors.report("host_session_shutdown", "persist clean host session failed", err)
	}
}

func (c *Collector) temperatureLoop(ctx context.Context) {
	ticker := time.NewTicker(c.intervals.temperature)
	defer ticker.Stop()
	var accumulator TemperatureAccumulator
	collect := func(at time.Time) {
		if accumulator.count > 0 && at.Sub(accumulator.start) >= c.intervals.temperatureBucket {
			if bucket, ok := accumulator.Flush(at); ok {
				c.enqueueMetric(writerMessage{batch: Batch{Temperatures: []TemperatureBucket{bucket}}})
			}
		}
		if temperature, ok := c.readTemperature(); ok {
			accumulator.Add(at, temperature.Celsius)
		}
	}
	collect(c.now().UTC())
	for {
		select {
		case <-ctx.Done():
			if bucket, ok := accumulator.Flush(c.now().UTC()); ok {
				c.enqueueFinalMetric(writerMessage{batch: Batch{Temperatures: []TemperatureBucket{bucket}}})
			}
			return
		case <-ticker.C:
			collect(c.now().UTC())
		}
	}
}

func (c *Collector) applicationLoop(ctx context.Context) {
	runPeriodic(ctx, c.intervals.applications, true, func() {
		sourceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		snapshot, err := c.deployments.Deployments(sourceCtx)
		if err != nil {
			c.errors.report("applications", "historical application collection failed", err)
			return
		}
		samples, events := c.appTracker.Observe(snapshot, c.now().UTC())
		c.enqueueMetric(writerMessage{batch: Batch{Applications: samples}})
		if len(events) > 0 {
			c.enqueueCritical(ctx, writerMessage{batch: Batch{Events: events}})
		}
	})
}

func (c *Collector) serviceLoop(ctx context.Context) {
	runPeriodic(ctx, c.intervals.services, true, func() {
		samples := c.health.check(ctx, c.now().UTC())
		events, state := c.serviceTracker.Observe(samples)
		c.enqueueMetric(writerMessage{batch: Batch{Services: samples}})
		if len(events) > 0 || len(state) > 0 {
			c.enqueueCritical(ctx, writerMessage{batch: Batch{Events: events, State: state}})
		}
	})
}

func (c *Collector) eventLoop(ctx context.Context) {
	runPeriodic(ctx, c.intervals.events, true, func() {
		sourceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		items, err := c.activities.Activity(sourceCtx)
		if err != nil {
			c.errors.report("events", "MiniBase activity ingestion failed", err)
			return
		}
		events := MiniBaseEvents(items)
		if len(events) > 0 {
			c.enqueueCritical(ctx, writerMessage{batch: Batch{Events: events}})
		}
	})
}

func (c *Collector) retentionLoop(ctx context.Context) {
	runPeriodic(ctx, c.intervals.retention, true, func() {
		cutoff := c.now().UTC().Add(-MetricRetention)
		c.enqueueCritical(ctx, writerMessage{pruneBefore: &cutoff})
	})
}

func (c *Collector) enqueueMetric(message writerMessage) {
	select {
	case c.metricQueue <- message:
	default:
		c.errors.report("metric_writer_queue", "observability metric writer queue full; sample dropped", nil)
	}
}

func (c *Collector) enqueueCritical(ctx context.Context, message writerMessage) bool {
	select {
	case c.criticalQueue <- message:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *Collector) enqueueFinalMetric(message writerMessage) {
	timer := time.NewTimer(finalMetricEnqueueTimeout)
	defer timer.Stop()
	select {
	case c.metricQueue <- message:
	case <-timer.C:
		c.errors.report("metric_writer_queue", "observability metric writer queue full; final temperature bucket dropped", nil)
	}
}

func (c *Collector) writeLoop(ctx context.Context) {
	metricQueue, criticalQueue := c.metricQueue, c.criticalQueue
	for metricQueue != nil || criticalQueue != nil {
		if criticalQueue != nil {
			select {
			case message, ok := <-criticalQueue:
				if !ok {
					criticalQueue = nil
					continue
				} else {
					c.writeMessage(ctx, message, true)
					continue
				}
			default:
			}
		}
		select {
		case <-ctx.Done():
			return
		case message, ok := <-criticalQueue:
			if !ok {
				criticalQueue = nil
				continue
			}
			c.writeMessage(ctx, message, true)
		case message, ok := <-metricQueue:
			if !ok {
				metricQueue = nil
				continue
			}
			c.writeMessage(ctx, message, false)
		}
	}
}

func (c *Collector) writeMessage(ctx context.Context, message writerMessage, critical bool) {
	attempts := 1
	if critical {
		attempts = criticalWriteAttempts
	}
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		operationContext, cancel := context.WithTimeout(ctx, writerOperationTimeout)
		if message.pruneBefore != nil {
			err = c.store.PruneMetrics(operationContext, *message.pruneBefore)
		} else {
			err = c.store.InsertBatch(operationContext, message.batch)
		}
		cancel()
		if err == nil {
			return
		}
		if attempt < attempts && !waitForRetry(ctx, c.retryDelay) {
			return
		}
	}
	if message.pruneBefore != nil {
		c.errors.report("retention", "observability retention failed", err)
	} else {
		c.errors.report("writer", "observability write failed", err)
	}
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func runPeriodic(ctx context.Context, interval time.Duration, immediate bool, fn func()) {
	if immediate {
		fn()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}

type healthService struct {
	id, name, url, unit string
}

type healthChecker struct {
	client   *http.Client
	services []healthService
}

func newHealthChecker(miniDeployURL, miniBaseURL, miniAIURL string) *healthChecker {
	return &healthChecker{
		client: &http.Client{Timeout: 2 * time.Second},
		services: []healthService{
			{id: "reactorlab", name: "ReactorLab", unit: "reactorlab.service"},
			{id: "minideploy", name: "MiniDeploy", url: miniDeployURL + "/health", unit: "minideploy.service"},
			{id: "minibase", name: "MiniBase", url: miniBaseURL + "/health", unit: "minibase.service"},
			{id: "miniai", name: "MiniAI", url: miniAIURL + "/health", unit: "miniai.service"},
		},
	}
}

func (h *healthChecker) check(ctx context.Context, at time.Time) []ServiceSample {
	units := make([]string, 0, len(h.services))
	for _, service := range h.services {
		units = append(units, service.unit)
	}
	identityCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	identities := system.ServiceInvocationIDs(identityCtx, units)
	cancel()
	result := make([]ServiceSample, len(h.services))
	var wg sync.WaitGroup
	for index, service := range h.services {
		index, service := index, service
		wg.Add(1)
		go func() {
			defer wg.Done()
			sample := ServiceSample{CollectedAt: at, ServiceID: service.id, Name: service.name,
				RestartIdentity: identities[service.unit], Status: "unavailable"}
			if service.id == "reactorlab" {
				sample.Available, sample.Status = true, "healthy"
				result[index] = sample
				return
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, service.url, nil)
			if err == nil {
				response, requestErr := h.client.Do(req)
				if requestErr == nil {
					_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
					_ = response.Body.Close()
					sample.Available = response.StatusCode >= 200 && response.StatusCode < 300
					if sample.Available {
						sample.Status = "healthy"
					} else {
						sample.Status = fmt.Sprintf("http_%d", response.StatusCode)
					}
				}
			}
			result[index] = sample
		}()
	}
	wg.Wait()
	return result
}

type errorReporter struct {
	mu       sync.Mutex
	last     map[string]time.Time
	interval time.Duration
}

func newErrorReporter(interval time.Duration) *errorReporter {
	return &errorReporter{last: make(map[string]time.Time), interval: interval}
}

func (r *errorReporter) report(key, message string, err error) {
	r.mu.Lock()
	now := time.Now()
	if previous := r.last[key]; !previous.IsZero() && now.Sub(previous) < r.interval {
		r.mu.Unlock()
		return
	}
	r.last[key] = now
	r.mu.Unlock()
	if err != nil {
		log.Printf("%s: %v", message, err)
	} else {
		log.Print(message)
	}
}

func normalizedURL(value, fallback string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return fallback
	}
	return value
}
