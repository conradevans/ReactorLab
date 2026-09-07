package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMigratesSchema(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "reactorlab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var version int
	if err := store.db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersion)
	}

	for _, table := range []string{
		"system_samples",
		"deployment_samples",
		"database_samples",
		"activity_events",
	} {
		var name string
		err := store.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}
}

func TestInsertBatchAndPrune(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "reactorlab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	oldTime := now.Add(-8 * 24 * time.Hour)
	recentTime := now.Add(-2 * time.Hour)

	backupAge := 3600.0
	for _, collectedAt := range []time.Time{oldTime, recentTime} {
		err := store.InsertBatch(ctx, Batch{
			System: SystemSample{
				CollectedAt:          collectedAt,
				CPUUsagePercent:      12.5,
				Load1:                0.8,
				Load5:                0.6,
				Load15:               0.4,
				MemoryUsedBytes:      4 * 1024 * 1024 * 1024,
				MemoryUsagePercent:   25,
				DiskUsedBytes:        18 * 1024 * 1024 * 1024,
				DiskUsagePercent:     2,
				TemperatureCelsius:   47.5,
				UptimeSeconds:        86400,
				NetworkRXBytesPerSec: 1200,
				NetworkTXBytesPerSec: 800,
			},
			Deployments: []DeploymentSample{
				{
					App:             "myscheduler",
					Status:          "healthy",
					ContainerCount:  2,
					CPUPercent:      1.5,
					MemoryUsedBytes: 256 * 1024 * 1024,
					RestartCount:    0,
					DatabaseState:   "linked",
				},
				{
					App:             "portfolio",
					Status:          "healthy",
					ContainerCount:  1,
					CPUPercent:      0.2,
					MemoryUsedBytes: 32 * 1024 * 1024,
					RestartCount:    0,
					DatabaseState:   "detached",
				},
			},
			Databases: []DatabaseSample{
				{
					ID:               "database_1",
					DisplayName:      "MyScheduler Production",
					Status:           "ready",
					SizeBytes:        9000000,
					Connections:      1,
					BackupCount:      5,
					BackupAgeSeconds: &backupAge,
					DeploymentState:  "linked",
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, occurredAt := range []time.Time{oldTime, recentTime} {
		if err := store.AppendActivity(ctx, ActivityEvent{
			OccurredAt:  occurredAt,
			Source:      "reactorlab",
			Kind:        "warning",
			Severity:    "warning",
			Subject:     "disk",
			Message:     "Disk usage crossed a warning threshold.",
			Fingerprint: "disk-warning",
		}); err != nil {
			t.Fatal(err)
		}
	}

	before, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before != (Counts{
		System:      2,
		Deployments: 4,
		Databases:   2,
		Activity:    2,
	}) {
		t.Fatalf("counts before prune = %#v", before)
	}

	if err := store.PruneBefore(ctx, now.Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	after, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after != (Counts{
		System:      1,
		Deployments: 2,
		Databases:   1,
		Activity:    1,
	}) {
		t.Fatalf("counts after prune = %#v", after)
	}
}

func TestInsertBatchRequiresTimestamp(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "reactorlab.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.InsertBatch(context.Background(), Batch{}); err == nil {
		t.Fatal("InsertBatch() error = nil")
	}
}
