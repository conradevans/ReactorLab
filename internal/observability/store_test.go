package observability

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "private", "observability.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func requireRecoveryEventIndex(t *testing.T, store *Store) {
	t.Helper()
	rows, err := store.db.Query("PRAGMA index_info(events_type_occurred_at)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var sequence, columnID int
		var name string
		if err := rows.Scan(&sequence, &columnID, &name); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0] != "event_type" || columns[1] != "occurred_at_ms" {
		t.Fatalf("recovery event index columns = %v", columns)
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestOpenCreatesSecureSchemaAndReopens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observability", "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var version int
	if err := store.db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d", version)
	}
	requireRecoveryEventIndex(t, store)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database mode = %o", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("directory mode = %o", got)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenMigratesV1ToCurrentWithoutLosingEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observability", "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	event := newEvent(
		"test",
		"existing-event",
		"host_restart",
		"host",
		"dell",
		"Dell host",
		time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
		"existing event",
		nil,
	)
	if err := store.InsertBatch(ctx, Batch{Events: []Event{event}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "DROP TABLE recovery_discord_outbox"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "DROP INDEX events_type_occurred_at"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version >= 2"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migrated.Close() })
	var version int
	if err := migrated.db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersion)
	}
	requireRecoveryEventIndex(t, migrated)
	var count int
	if err := migrated.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE event_id = ?", event.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("preserved event count = %d, want 1", count)
	}
}

func TestOpenRejectsNonRegularDatabase(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("expected directory database path to fail")
	}
}

func TestInsertAndQueryAllMetricKinds(t *testing.T) {
	store := openTestStore(t)
	query := NewQueryService(store)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	batch := Batch{
		Hosts: []HostSample{{
			CollectedAt: now, CPUPercent: floatPointer(42), MemoryUsedBytes: 40,
			MemoryTotalBytes: 100, Load1: 1, Load5: 2, Load15: 3,
			DiskUsedBytes: 60, DiskTotalBytes: 100, DiskReadBPS: floatPointer(10),
			DiskWriteBPS: floatPointer(20), NetworkRXBPS: floatPointer(30),
			NetworkTXBPS: floatPointer(40), UptimeSeconds: 1000, BootID: "boot-a",
		}},
		Temperatures: []TemperatureBucket{{
			BucketStart: now, BucketEnd: now.Add(5 * time.Second), SampleCount: 5,
			MinCelsius: 58, AvgCelsius: 72.2, MaxCelsius: 91,
			PeakAt: now.Add(3 * time.Second),
		}},
		Applications: []ApplicationSample{{
			CollectedAt: now, AppID: "app-a", Name: "App A", Status: "healthy",
			CPUPercent: 10, MemoryUsedBytes: 20, MemoryLimitBytes: 100,
			NetworkRXBPS: floatPointer(5), NetworkTXBPS: floatPointer(6), RestartCount: 2,
		}},
		Services: []ServiceSample{{
			CollectedAt: now, ServiceID: "minibase", Name: "MiniBase",
			Available: true, Status: "healthy", RestartIdentity: "inv-a",
		}},
	}
	if err := store.InsertBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	window := Range{Name: "15m", From: now.Add(-time.Minute), To: now.Add(time.Minute), Bucket: 5 * time.Second, MaxPoints: 720}
	host, err := query.QueryHost(context.Background(), window)
	if err != nil || len(host) != 1 || host[0].CPUAverage == nil || *host[0].CPUAverage != 42 {
		t.Fatalf("host = %#v, err = %v", host, err)
	}
	temperature, err := query.QueryTemperature(context.Background(), window)
	if err != nil || len(temperature) != 1 || temperature[0].MaxCelsius != 91 || !temperature[0].PeakAt.Equal(now.Add(3*time.Second)) {
		t.Fatalf("temperature = %#v, err = %v", temperature, err)
	}
	apps, err := query.QueryApplications(context.Background(), window)
	if err != nil || len(apps) != 1 || apps[0].ID != "app-a" || apps[0].RestartCount != 2 {
		t.Fatalf("apps = %#v, err = %v", apps, err)
	}
	app, err := query.QueryApplication(context.Background(), "app-a", window)
	if err != nil || len(app) != 1 || app[0].NetworkRXAverage == nil || *app[0].NetworkRXAverage != 5 {
		t.Fatalf("app = %#v, err = %v", app, err)
	}
	services, err := query.QueryServices(context.Background(), window)
	if err != nil || len(services) != 1 || !services[0].Points[0].Available {
		t.Fatalf("services = %#v, err = %v", services, err)
	}
}

func TestTemperatureDownsamplingPreservesWeightedAverageAndPeak(t *testing.T) {
	store := openTestStore(t)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	buckets := []TemperatureBucket{
		{BucketStart: now, BucketEnd: now.Add(5 * time.Second), SampleCount: 5, MinCelsius: 58, AvgCelsius: 60, MaxCelsius: 91, PeakAt: now.Add(2 * time.Second)},
		{BucketStart: now.Add(5 * time.Second), BucketEnd: now.Add(10 * time.Second), SampleCount: 3, MinCelsius: 61, AvgCelsius: 70, MaxCelsius: 75, PeakAt: now.Add(8 * time.Second)},
	}
	if err := store.InsertBatch(context.Background(), Batch{Temperatures: buckets}); err != nil {
		t.Fatal(err)
	}
	points, err := NewQueryService(store).QueryTemperature(context.Background(), Range{
		Name: "test", From: now, To: now.Add(time.Minute), Bucket: time.Minute, MaxPoints: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].MaxCelsius != 91 || !points[0].PeakAt.Equal(now.Add(2*time.Second)) {
		t.Fatalf("peak was not preserved: %#v", points)
	}
	if want := 63.75; math.Abs(points[0].AvgCelsius-want) > 0.001 {
		t.Fatalf("average = %v, want %v", points[0].AvgCelsius, want)
	}
}

func TestRangesAreBoundedAndInvalidRangesFail(t *testing.T) {
	now := time.Now().UTC()
	for _, name := range []string{"15m", "1h", "6h", "24h", "7d"} {
		window, err := ResolveRange(name, now)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if points := int(window.To.Sub(window.From) / window.Bucket); points > MaxDisplayPoints {
			t.Fatalf("%s produces %d points", name, points)
		}
	}
	if _, err := ResolveRange("30d", now); err == nil {
		t.Fatal("expected invalid preset error")
	}
	if err := ValidateWindow(now.Add(-MetricRetention-time.Second), now, 100); err == nil {
		t.Fatal("expected excessive lookback error")
	}
	if err := ValidateWindow(now.Add(-time.Hour), now, MaxDisplayPoints+1); err == nil {
		t.Fatal("expected excessive point bound error")
	}
}

func TestRetentionDeletesOnlyMetrics(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.Add(-MetricRetention - time.Minute)
	rule := AlertRule{ID: "rule-1", Scope: "host", Metric: "cpu_percent", Operator: ">", Threshold: 90, DurationSeconds: 300, Enabled: true, CreatedAt: old, UpdatedAt: old}
	if err := store.PutAlertRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	incident := AlertIncident{ID: "incident-1", RuleID: rule.ID, ResourceType: "host", OpenedAt: old, LastObservedAt: old, State: "open"}
	if err := store.PutAlertIncident(ctx, incident); err != nil {
		t.Fatal(err)
	}
	event := newEvent("test", "event-1", "host_restart", "host", "dell", "Dell host", old, "old event", nil)
	if err := store.InsertBatch(ctx, Batch{
		Hosts:        []HostSample{{CollectedAt: old, MemoryTotalBytes: 1, DiskTotalBytes: 1}},
		Temperatures: []TemperatureBucket{{BucketStart: old, BucketEnd: old.Add(time.Second), SampleCount: 1, MinCelsius: 60, AvgCelsius: 60, MaxCelsius: 60, PeakAt: old}},
		Applications: []ApplicationSample{{CollectedAt: old, AppID: "a", Name: "A", Status: "healthy"}},
		Services:     []ServiceSample{{CollectedAt: old, ServiceID: "s", Name: "S", Status: "healthy", Available: true}},
		Events:       []Event{event},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.PruneMetrics(ctx, now.Add(-MetricRetention)); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"host_samples", "temperature_buckets", "application_samples", "service_samples"} {
		var count int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s count = %d, err = %v", table, count, err)
		}
	}
	for _, table := range []string{"events", "alert_rules", "alert_incidents"} {
		var count int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count = %d, err = %v", table, count, err)
		}
	}
}

func TestEventDeduplicationUsesSourceIdentity(t *testing.T) {
	store := openTestStore(t)
	event := newEvent("minibase", "upstream-1", "database_backup", "database", "db-a", "A", time.Now(), "Backup created", nil)
	if err := store.InsertBatch(context.Background(), Batch{Events: []Event{event, event}}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertBatch(context.Background(), Batch{Events: []Event{event}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil || count != 1 {
		t.Fatalf("events = %d, err = %v", count, err)
	}
}
