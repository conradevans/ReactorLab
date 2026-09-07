package metrics

import (
	"context"
	"log"
	"time"

	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

const (
	DefaultInterval  = time.Minute
	DefaultRetention = 7 * 24 * time.Hour
	sourceTimeout    = 8 * time.Second
)

type historyStore interface {
	InsertBatch(context.Context, history.Batch) error
	AppendActivity(context.Context, history.ActivityEvent) error
	LatestActivity(context.Context, string) (history.ActivityEvent, bool, error)
	PruneBefore(context.Context, time.Time) error
}

type deploymentSource interface {
	Deployments(context.Context) (minideploy.Snapshot, error)
}

type databaseSource interface {
	Databases(context.Context) (minibase.Snapshot, error)
}

type systemCollector func() (reactorsystem.Metrics, error)

type Sampler struct {
	store         historyStore
	deployments   deploymentSource
	databases     databaseSource
	collectSystem systemCollector
	interval      time.Duration
	retention     time.Duration
	now           func() time.Time
}

func NewSampler(store *history.Store) *Sampler {
	return &Sampler{
		store:         store,
		deployments:   minideploy.NewClient(minideploy.DefaultBaseURL, 5*time.Second),
		databases:     minibase.NewClient(minibase.DefaultBaseURL, 5*time.Second),
		collectSystem: reactorsystem.Collect,
		interval:      DefaultInterval,
		retention:     DefaultRetention,
		now:           time.Now,
	}
}

func (s *Sampler) Run(ctx context.Context) {
	if err := s.sampleOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("ReactorLab history sample failed: %v", err)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.sampleOnce(ctx); err != nil && ctx.Err() == nil {
				log.Printf("ReactorLab history sample failed: %v", err)
			}
		}
	}
}

func (s *Sampler) sampleOnce(ctx context.Context) error {
	systemMetrics, err := s.collectSystem()
	if err != nil {
		return err
	}

	type deploymentResult struct {
		snapshot  minideploy.Snapshot
		available bool
	}
	type databaseResult struct {
		snapshot  minibase.Snapshot
		available bool
	}

	deploymentResults := make(chan deploymentResult, 1)
	databaseResults := make(chan databaseResult, 1)

	go func() {
		snapshot, available := s.collectDeployments(ctx)
		deploymentResults <- deploymentResult{snapshot: snapshot, available: available}
	}()
	go func() {
		snapshot, available := s.collectDatabases(ctx)
		databaseResults <- databaseResult{snapshot: snapshot, available: available}
	}()

	deploymentResultValue := <-deploymentResults
	databaseResultValue := <-databaseResults

	deploymentSnapshot := deploymentResultValue.snapshot
	deploymentsAvailable := deploymentResultValue.available
	databaseSnapshot := databaseResultValue.snapshot
	databasesAvailable := databaseResultValue.available

	batch := history.Batch{
		System: history.SystemSample{
			CollectedAt:          systemMetrics.CollectedAt,
			CPUUsagePercent:      systemMetrics.CPU.UsagePercent,
			Load1:                systemMetrics.CPU.Load1,
			Load5:                systemMetrics.CPU.Load5,
			Load15:               systemMetrics.CPU.Load15,
			MemoryUsedBytes:      systemMetrics.Memory.UsedBytes,
			MemoryUsagePercent:   systemMetrics.Memory.UsagePercent,
			DiskUsedBytes:        systemMetrics.Disk.UsedBytes,
			DiskUsagePercent:     systemMetrics.Disk.UsagePercent,
			TemperatureCelsius:   systemMetrics.Temperature.Celsius,
			UptimeSeconds:        systemMetrics.UptimeSeconds,
			NetworkRXBytesPerSec: systemMetrics.Network.RXBytesPerSec,
			NetworkTXBytesPerSec: systemMetrics.Network.TXBytesPerSec,
		},
		Deployments: deploymentSamples(
			deploymentSnapshot,
			databaseSnapshot,
			databasesAvailable,
		),
		Databases: databaseSamples(
			databaseSnapshot,
			deploymentSnapshot,
			deploymentsAvailable,
		),
	}

	if err := s.store.InsertBatch(ctx, batch); err != nil {
		return err
	}

	if err := s.evaluateWarnings(
		ctx,
		batch,
		deploymentsAvailable,
		databasesAvailable,
	); err != nil && ctx.Err() == nil {
		log.Printf("ReactorLab warning evaluation failed: %v", err)
	}

	cutoff := s.now().UTC().Add(-s.retention)
	if err := s.store.PruneBefore(ctx, cutoff); err != nil {
		log.Printf("ReactorLab history prune failed: %v", err)
	}

	return nil
}

func (s *Sampler) collectDeployments(ctx context.Context) (minideploy.Snapshot, bool) {
	if s.deployments == nil {
		return minideploy.Snapshot{}, false
	}

	sourceCtx, cancel := context.WithTimeout(ctx, sourceTimeout)
	defer cancel()

	snapshot, err := s.deployments.Deployments(sourceCtx)
	if err != nil {
		log.Printf("MiniDeploy history source unavailable: %v", err)
		return minideploy.Snapshot{}, false
	}
	return snapshot, true
}

func (s *Sampler) collectDatabases(ctx context.Context) (minibase.Snapshot, bool) {
	if s.databases == nil {
		return minibase.Snapshot{}, false
	}

	sourceCtx, cancel := context.WithTimeout(ctx, sourceTimeout)
	defer cancel()

	snapshot, err := s.databases.Databases(sourceCtx)
	if err != nil {
		log.Printf("MiniBase history source unavailable: %v", err)
		return minibase.Snapshot{}, false
	}
	return snapshot, true
}

func deploymentSamples(
	deployments minideploy.Snapshot,
	databases minibase.Snapshot,
	databasesAvailable bool,
) []history.DeploymentSample {
	if deployments.Deployments == nil {
		return []history.DeploymentSample{}
	}

	output := make([]history.DeploymentSample, 0, len(deployments.Deployments))
	for _, deployment := range deployments.Deployments {
		var cpu float64
		var memory uint64
		var restarts int

		for _, container := range deployment.Containers {
			cpu += container.CPUPercent
			memory += container.MemoryUsedBytes
			restarts += container.RestartCount
		}

		databaseState := "unavailable"
		if databasesAvailable {
			databaseState = databaseStateForApp(deployment.App, databases)
		}

		output = append(output, history.DeploymentSample{
			App:             deployment.App,
			Status:          deployment.Status,
			ContainerCount:  len(deployment.Containers),
			CPUPercent:      cpu,
			MemoryUsedBytes: memory,
			RestartCount:    restarts,
			DatabaseState:   databaseState,
		})
	}
	return output
}

func databaseSamples(
	databases minibase.Snapshot,
	deployments minideploy.Snapshot,
	deploymentsAvailable bool,
) []history.DatabaseSample {
	if databases.Databases == nil {
		return []history.DatabaseSample{}
	}

	output := make([]history.DatabaseSample, 0, len(databases.Databases))
	for _, database := range databases.Databases {
		deploymentState := "unavailable"
		if deploymentsAvailable {
			deploymentState = deploymentStateForDatabase(database, deployments)
		}

		output = append(output, history.DatabaseSample{
			ID:               database.ID,
			DisplayName:      database.DisplayName,
			Status:           database.Status,
			SizeBytes:        database.SizeBytes,
			Connections:      database.Connections,
			BackupCount:      database.BackupCount,
			BackupAgeSeconds: database.BackupAgeSeconds,
			DeploymentState:  deploymentState,
		})
	}
	return output
}

func databaseStateForApp(app string, snapshot minibase.Snapshot) string {
	matches := make(map[string]struct{})

	for _, database := range snapshot.Databases {
		for _, attachment := range database.Attachments {
			if attachment.ConsumerType == "minideploy" &&
				attachment.BindingName == "primary" &&
				attachment.ConsumerRef == app {
				matches[database.ID] = struct{}{}
			}
		}
	}

	switch len(matches) {
	case 0:
		return "detached"
	case 1:
		return "linked"
	default:
		return "conflict"
	}
}

func deploymentStateForDatabase(
	database minibase.Database,
	snapshot minideploy.Snapshot,
) string {
	consumerRefs := make(map[string]struct{})

	for _, attachment := range database.Attachments {
		if attachment.ConsumerType == "minideploy" &&
			attachment.BindingName == "primary" {
			consumerRefs[attachment.ConsumerRef] = struct{}{}
		}
	}

	switch len(consumerRefs) {
	case 0:
		return "detached"
	case 1:
	default:
		return "conflict"
	}

	for app := range consumerRefs {
		if _, ok := minideploy.FindDeployment(snapshot, app); ok {
			return "linked"
		}
	}
	return "unresolved"
}
