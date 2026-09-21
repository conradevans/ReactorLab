package observability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/system"
)

type collectorStoreSpy struct {
	mu             sync.Mutex
	insertFailures int
	pruneFailures  int
	insertCalls    int
	pruneCalls     int
	batches        []Batch
	prunes         []time.Time
	state          map[string]string
}

func (s *collectorStoreSpy) InsertBatch(_ context.Context, batch Batch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.insertCalls++
	if s.insertFailures > 0 {
		s.insertFailures--
		return errors.New("transient insert failure")
	}
	s.batches = append(s.batches, batch)
	if s.state == nil {
		s.state = make(map[string]string)
	}
	for key, value := range batch.State {
		s.state[key] = value
	}
	return nil
}

func (s *collectorStoreSpy) PruneMetrics(_ context.Context, before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneCalls++
	if s.pruneFailures > 0 {
		s.pruneFailures--
		return errors.New("transient prune failure")
	}
	s.prunes = append(s.prunes, before)
	return nil
}

func (s *collectorStoreSpy) State(_ context.Context, key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.state[key]
	return value, ok, nil
}

func newCollectorForWriterTest(store collectorStore, metricCapacity, criticalCapacity int) *Collector {
	return &Collector{
		store:         store,
		metricQueue:   make(chan writerMessage, metricCapacity),
		criticalQueue: make(chan writerMessage, criticalCapacity),
		retryDelay:    0,
		errors:        newErrorReporter(0),
	}
}

func closeAndDrainCollector(collector *Collector) {
	close(collector.metricQueue)
	close(collector.criticalQueue)
	collector.writeLoop(context.Background())
}

func TestMetricQueueDropsSamplesWhenSaturated(t *testing.T) {
	collector := newCollectorForWriterTest(&collectorStoreSpy{}, 1, 1)
	first := writerMessage{batch: Batch{Hosts: []HostSample{{BootID: "first"}}}}
	second := writerMessage{batch: Batch{Hosts: []HostSample{{BootID: "second"}}}}

	collector.enqueueMetric(first)
	collector.enqueueMetric(second)

	if len(collector.metricQueue) != 1 {
		t.Fatalf("metric queue length = %d, want 1", len(collector.metricQueue))
	}
	got := <-collector.metricQueue
	if got.batch.Hosts[0].BootID != "first" {
		t.Fatalf("queued metric = %q, want first", got.batch.Hosts[0].BootID)
	}
}

func TestCriticalTransitionsStateAndRetentionSurviveMetricPressure(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	old := now.Add(-MetricRetention - time.Hour)
	if err := store.InsertBatch(context.Background(), Batch{
		Hosts: []HostSample{{CollectedAt: old, BootID: "old"}},
	}); err != nil {
		t.Fatal(err)
	}

	collector := newCollectorForWriterTest(store, 1, 8)
	collector.enqueueMetric(writerMessage{batch: Batch{
		Hosts: []HostSample{{CollectedAt: now, BootID: "current"}},
	}})
	collector.enqueueMetric(writerMessage{batch: Batch{
		Hosts: []HostSample{{CollectedAt: now.Add(time.Second), BootID: "dropped"}},
	}})

	var serviceTracker ServiceTracker
	serviceTracker.SetHealthBaseline("minibase", true)
	serviceTracker.SetInvocationBaseline("minibase", "inv-a")
	outageEvents, outageState := serviceTracker.Observe([]ServiceSample{{
		CollectedAt: now, ServiceID: "minibase", Name: "MiniBase",
		Available: false, Status: "unavailable", RestartIdentity: "inv-a",
	}})
	collector.enqueueCritical(context.Background(), writerMessage{batch: Batch{
		Events: outageEvents, State: outageState,
	}})
	repeatedEvents, repeatedState := serviceTracker.Observe([]ServiceSample{{
		CollectedAt: now.Add(time.Second), ServiceID: "minibase", Name: "MiniBase",
		Available: false, Status: "unavailable", RestartIdentity: "inv-a",
	}})
	if len(repeatedEvents) != 0 || len(repeatedState) != 0 {
		t.Fatalf("repeated outage produced events=%d state=%d", len(repeatedEvents), len(repeatedState))
	}
	recoveryEvents, recoveryState := serviceTracker.Observe([]ServiceSample{{
		CollectedAt: now.Add(2 * time.Second), ServiceID: "minibase", Name: "MiniBase",
		Available: true, Status: "healthy", RestartIdentity: "inv-a",
	}})
	collector.enqueueCritical(context.Background(), writerMessage{batch: Batch{
		Events: recoveryEvents, State: recoveryState,
	}})

	var appTracker ApplicationTracker
	appSnapshot := func(restarts int, at time.Time) minideploy.Snapshot {
		return minideploy.Snapshot{
			CollectedAt: at,
			Deployments: []minideploy.Deployment{{
				App: "alpha", Status: "healthy",
				Containers: []minideploy.ContainerMetrics{{
					Container: "alpha-1", RestartCount: restarts,
				}},
			}},
		}
	}
	appTracker.Observe(appSnapshot(0, now), now)
	_, appEvents := appTracker.Observe(appSnapshot(1, now.Add(3*time.Second)), now.Add(3*time.Second))
	collector.enqueueCritical(context.Background(), writerMessage{batch: Batch{Events: appEvents}})

	var hostTracker HostTracker
	hostTracker.SetBootBaseline("boot-a")
	_, hostEvents, hostState := hostTracker.Observe(system.HistoricalSnapshot{
		ObservedAt: now.Add(4 * time.Second), BootID: "boot-b",
	})
	collector.enqueueCritical(context.Background(), writerMessage{batch: Batch{
		Events: hostEvents, State: hostState,
	}})

	cutoff := now.Add(-MetricRetention)
	collector.enqueueCritical(context.Background(), writerMessage{pruneBefore: &cutoff})
	closeAndDrainCollector(collector)

	events, err := NewQueryService(store).QueryEvents(
		context.Background(), now.Add(-time.Minute), now.Add(time.Minute), 100,
	)
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	for _, event := range events {
		counts[event.Type]++
	}
	for _, eventType := range []string{
		"service_outage", "service_recovery", "application_restart", "host_restart",
	} {
		if counts[eventType] != 1 {
			t.Fatalf("%s count = %d, want 1; events = %#v", eventType, counts[eventType], events)
		}
	}
	if value, ok, err := store.State(context.Background(), serviceStatePrefix+"minibase"); err != nil || !ok || value != "true" {
		t.Fatalf("service state = %q, %t, %v", value, ok, err)
	}
	if value, ok, err := store.State(context.Background(), hostBootStateKey); err != nil || !ok || value != "boot-b" {
		t.Fatalf("host state = %q, %t, %v", value, ok, err)
	}
	var oldCount int
	if err := store.db.QueryRow(
		"SELECT COUNT(*) FROM host_samples WHERE collected_at_ms = ?", old.UnixMilli(),
	).Scan(&oldCount); err != nil {
		t.Fatal(err)
	}
	if oldCount != 0 {
		t.Fatalf("old metric count = %d, want 0", oldCount)
	}
}

func TestCriticalWriterRetriesTransientFailureAndBoundsRetries(t *testing.T) {
	t.Run("event retries after transient failure", func(t *testing.T) {
		store := &collectorStoreSpy{insertFailures: 1}
		collector := newCollectorForWriterTest(store, 1, 1)
		collector.writeMessage(context.Background(), writerMessage{
			batch: Batch{Events: []Event{{ID: "event"}}},
		}, true)
		if store.insertCalls != 2 || len(store.batches) != 1 {
			t.Fatalf("insert calls = %d, persisted batches = %d", store.insertCalls, len(store.batches))
		}
	})

	t.Run("retry count is bounded", func(t *testing.T) {
		store := &collectorStoreSpy{insertFailures: 10}
		collector := newCollectorForWriterTest(store, 1, 1)
		collector.writeMessage(context.Background(), writerMessage{batch: Batch{
			State: map[string]string{"key": "value"},
		}}, true)
		if store.insertCalls != criticalWriteAttempts {
			t.Fatalf("insert calls = %d, want %d", store.insertCalls, criticalWriteAttempts)
		}
	})
}

func TestRetentionRequestRetriesDespiteMetricPressure(t *testing.T) {
	store := &collectorStoreSpy{pruneFailures: 1}
	collector := newCollectorForWriterTest(store, 1, 1)
	collector.enqueueMetric(writerMessage{batch: Batch{Hosts: []HostSample{{BootID: "metric"}}}})
	cutoff := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if !collector.enqueueCritical(context.Background(), writerMessage{pruneBefore: &cutoff}) {
		t.Fatal("critical retention enqueue was not accepted")
	}
	closeAndDrainCollector(collector)
	if store.pruneCalls != 2 || len(store.prunes) != 1 || !store.prunes[0].Equal(cutoff) {
		t.Fatalf("prune calls = %d, successful prunes = %#v", store.pruneCalls, store.prunes)
	}
}

type ambiguousCommitStore struct {
	*Store
	mu       sync.Mutex
	returned bool
}

func (s *ambiguousCommitStore) InsertBatch(ctx context.Context, batch Batch) error {
	if err := s.Store.InsertBatch(ctx, batch); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.returned {
		s.returned = true
		return errors.New("ambiguous commit result")
	}
	return nil
}

func TestCriticalRetryDoesNotDuplicateEventAfterAmbiguousCommit(t *testing.T) {
	store := openTestStore(t)
	wrapped := &ambiguousCommitStore{Store: store}
	collector := newCollectorForWriterTest(wrapped, 1, 1)
	event := newEvent(
		"reactorlab", "durable-1", "host_restart", "host", "dell", "Dell host",
		time.Now().UTC(), "Dell host restarted", nil,
	)
	collector.writeMessage(context.Background(), writerMessage{
		batch: Batch{Events: []Event{event}},
	}, true)

	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM events WHERE event_id = ?", event.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
}

func TestPendingCriticalEnqueueStopsWhenContextIsCancelled(t *testing.T) {
	collector := newCollectorForWriterTest(&collectorStoreSpy{}, 1, 1)
	collector.criticalQueue <- writerMessage{batch: Batch{State: map[string]string{"first": "1"}}}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan bool)
	go func() {
		close(started)
		done <- collector.enqueueCritical(ctx, writerMessage{
			batch: Batch{State: map[string]string{"second": "2"}},
		})
	}()
	<-started
	cancel()
	select {
	case accepted := <-done:
		if accepted {
			t.Fatal("cancelled critical enqueue was accepted into a full queue")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled critical enqueue did not return")
	}
}

type emptyDeploymentSource struct{}

func (emptyDeploymentSource) Deployments(context.Context) (minideploy.Snapshot, error) {
	return minideploy.Snapshot{}, nil
}

type emptyActivitySource struct{}

func (emptyActivitySource) Activity(context.Context) ([]minibase.Activity, error) {
	return nil, nil
}

func TestCollectorShutdownWithPendingProducersIsBoundedAndClosesAfterSenders(t *testing.T) {
	store := &collectorStoreSpy{}
	collector := newCollectorForWriterTest(store, 1, 1)
	collector.deployments = emptyDeploymentSource{}
	collector.activities = emptyActivitySource{}
	collector.health = &healthChecker{}
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	collector.now = func() time.Time { return now }
	collector.readHost = func(at time.Time) (system.HistoricalSnapshot, error) {
		return system.HistoricalSnapshot{ObservedAt: at, BootID: "boot-a"}, nil
	}
	collector.readTemperature = func() (system.TemperatureStats, bool) {
		return system.TemperatureStats{}, false
	}
	collector.intervals = collectorIntervals{
		host: time.Hour, temperature: time.Hour, temperatureBucket: time.Hour,
		applications: time.Hour, services: time.Hour, events: time.Hour, retention: time.Hour,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		collector.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector shutdown exceeded bounded test deadline")
	}

	value, found, err := store.State(context.Background(), hostSessionStateKey)
	if err != nil || !found {
		t.Fatalf("clean host session found=%v err=%v", found, err)
	}
	session, err := decodeHostSession(value)
	if err != nil {
		t.Fatal(err)
	}
	if !session.CleanShutdown {
		t.Fatalf("shutdown session = %#v, want clean", session)
	}
}
