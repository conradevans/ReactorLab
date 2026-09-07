package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

type fakeStore struct {
	batches []history.Batch
	prunes  []time.Time
}

func (f *fakeStore) InsertBatch(_ context.Context, batch history.Batch) error {
	f.batches = append(f.batches, batch)
	return nil
}

func (f *fakeStore) PruneBefore(_ context.Context, cutoff time.Time) error {
	f.prunes = append(f.prunes, cutoff)
	return nil
}

type fakeDeployments struct {
	snapshot minideploy.Snapshot
	err      error
}

func (f fakeDeployments) Deployments(context.Context) (minideploy.Snapshot, error) {
	return f.snapshot, f.err
}

type fakeDatabases struct {
	snapshot minibase.Snapshot
	err      error
}

func (f fakeDatabases) Databases(context.Context) (minibase.Snapshot, error) {
	return f.snapshot, f.err
}

func fixedSystemSample(collectedAt time.Time) (reactorsystem.Metrics, error) {
	return reactorsystem.Metrics{
		CPU: reactorsystem.CPUStats{
			UsagePercent: 12.5,
			Load1:        0.2,
			Load5:        0.3,
			Load15:       0.4,
		},
		Memory: reactorsystem.MemoryStats{
			UsedBytes:    4000,
			UsagePercent: 25,
		},
		Disk: reactorsystem.DiskStats{
			UsedBytes:    8000,
			UsagePercent: 2,
		},
		Temperature:   reactorsystem.TemperatureStats{Celsius: 45},
		UptimeSeconds: 900,
		Network: reactorsystem.NetworkStats{
			RXBytesPerSec: 100,
			TXBytesPerSec: 200,
		},
		CollectedAt: collectedAt,
	}, nil
}

func TestSampleOnceStoresAvailableSourcesAndRelationships(t *testing.T) {
	now := time.Date(2026, 9, 7, 9, 30, 0, 0, time.UTC)
	backupAge := 60.0
	store := &fakeStore{}

	sampler := &Sampler{
		store: store,
		deployments: fakeDeployments{
			snapshot: minideploy.Snapshot{
				Deployments: []minideploy.Deployment{
					{
						App:    "myscheduler",
						Status: "healthy",
						Containers: []minideploy.ContainerMetrics{
							{
								CPUPercent:      2.5,
								MemoryUsedBytes: 1000,
								RestartCount:    1,
							},
							{
								CPUPercent:      1.5,
								MemoryUsedBytes: 2000,
								RestartCount:    2,
							},
						},
					},
					{
						App:    "portfolio",
						Status: "healthy",
					},
				},
			},
		},
		databases: fakeDatabases{
			snapshot: minibase.Snapshot{
				Databases: []minibase.Database{
					{
						ID:               "database_1",
						DisplayName:      "MyScheduler Production",
						Status:           "ready",
						SizeBytes:        5000,
						Connections:      1,
						BackupCount:      5,
						BackupAgeSeconds: &backupAge,
						Attachments: []minibase.Attachment{
							{
								ConsumerType: "minideploy",
								ConsumerRef:  "myscheduler",
								BindingName:  "primary",
							},
						},
					},
				},
			},
		},
		collectSystem: func() (reactorsystem.Metrics, error) {
			return fixedSystemSample(now)
		},
		retention: DefaultRetention,
		now:       func() time.Time { return now },
	}

	if err := sampler.sampleOnce(context.Background()); err != nil {
		t.Fatalf("sampleOnce() error = %v", err)
	}

	if len(store.batches) != 1 {
		t.Fatalf("batch count = %d", len(store.batches))
	}
	batch := store.batches[0]

	if batch.System.CPUUsagePercent != 12.5 ||
		batch.System.MemoryUsedBytes != 4000 ||
		batch.System.NetworkTXBytesPerSec != 200 {
		t.Fatalf("system = %#v", batch.System)
	}

	if len(batch.Deployments) != 2 {
		t.Fatalf("deployments = %#v", batch.Deployments)
	}
	if got := batch.Deployments[0]; got.DatabaseState != "linked" ||
		got.ContainerCount != 2 ||
		got.CPUPercent != 4 ||
		got.MemoryUsedBytes != 3000 ||
		got.RestartCount != 3 {
		t.Fatalf("myscheduler sample = %#v", got)
	}
	if got := batch.Deployments[1]; got.DatabaseState != "detached" {
		t.Fatalf("portfolio sample = %#v", got)
	}

	if len(batch.Databases) != 1 {
		t.Fatalf("databases = %#v", batch.Databases)
	}
	if got := batch.Databases[0]; got.DeploymentState != "linked" ||
		got.ID != "database_1" ||
		got.BackupCount != 5 {
		t.Fatalf("database sample = %#v", got)
	}

	if len(store.prunes) != 1 {
		t.Fatalf("prune count = %d", len(store.prunes))
	}
	wantCutoff := now.Add(-DefaultRetention)
	if !store.prunes[0].Equal(wantCutoff) {
		t.Fatalf("cutoff = %v, want %v", store.prunes[0], wantCutoff)
	}
}

func TestSampleOnceKeepsSystemWhenSourcesUnavailable(t *testing.T) {
	now := time.Date(2026, 9, 7, 9, 30, 0, 0, time.UTC)
	store := &fakeStore{}

	sampler := &Sampler{
		store:       store,
		deployments: fakeDeployments{err: errors.New("down")},
		databases:   fakeDatabases{err: errors.New("down")},
		collectSystem: func() (reactorsystem.Metrics, error) {
			return fixedSystemSample(now)
		},
		retention: DefaultRetention,
		now:       func() time.Time { return now },
	}

	if err := sampler.sampleOnce(context.Background()); err != nil {
		t.Fatalf("sampleOnce() error = %v", err)
	}

	if len(store.batches) != 1 {
		t.Fatalf("batch count = %d", len(store.batches))
	}
	batch := store.batches[0]
	if len(batch.Deployments) != 0 || len(batch.Databases) != 0 {
		t.Fatalf("partial batch = %#v", batch)
	}
	if batch.System.CollectedAt != now {
		t.Fatalf("system collected at = %v", batch.System.CollectedAt)
	}
}

func TestRelationshipStates(t *testing.T) {
	databases := minibase.Snapshot{
		Databases: []minibase.Database{
			{
				ID: "database_a",
				Attachments: []minibase.Attachment{
					{
						ConsumerType: "minideploy",
						ConsumerRef:  "app",
						BindingName:  "primary",
					},
				},
			},
		},
	}

	if got := databaseStateForApp("app", databases); got != "linked" {
		t.Fatalf("database state = %q", got)
	}
	if got := databaseStateForApp("other", databases); got != "detached" {
		t.Fatalf("database state = %q", got)
	}

	deployments := minideploy.Snapshot{
		Deployments: []minideploy.Deployment{{App: "app"}},
	}
	if got := deploymentStateForDatabase(databases.Databases[0], deployments); got != "linked" {
		t.Fatalf("deployment state = %q", got)
	}

	missing := databases.Databases[0]
	missing.Attachments[0].ConsumerRef = "missing"
	if got := deploymentStateForDatabase(missing, deployments); got != "unresolved" {
		t.Fatalf("deployment state = %q", got)
	}
}

func TestSampleOnceFailsWhenSystemUnavailable(t *testing.T) {
	store := &fakeStore{}
	sampler := &Sampler{
		store: store,
		collectSystem: func() (reactorsystem.Metrics, error) {
			return reactorsystem.Metrics{}, errors.New("system down")
		},
		retention: DefaultRetention,
		now:       time.Now,
	}

	if err := sampler.sampleOnce(context.Background()); err == nil {
		t.Fatal("sampleOnce() error = nil")
	}
	if len(store.batches) != 0 {
		t.Fatalf("unexpected batches = %#v", store.batches)
	}
}
