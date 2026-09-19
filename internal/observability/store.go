package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const maxEventDetailsBytes = 4096

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("observability database path is required")
	}
	if err := secureDatabasePath(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open observability database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure observability database: %w", err)
	}
	return store, nil
}

func secureDatabasePath(path string) error {
	parent := filepath.Dir(path)
	if parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return fmt.Errorf("create observability directory: %w", err)
		}
		if err := os.Chmod(parent, 0o700); err != nil {
			return fmt.Errorf("secure observability directory: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		return file.Close()
	}
	if !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create observability database: %w", err)
	}
	info, statErr := os.Lstat(path)
	if statErr != nil {
		return fmt.Errorf("inspect observability database: %w", statErr)
	}
	if !info.Mode().IsRegular() {
		return errors.New("observability database must be a regular file")
	}
	return os.Chmod(path, 0o600)
}

func (s *Store) configure(ctx context.Context) error {
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure observability database: %w", err)
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin observability migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at_ms INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("create observability migrations: %w", err)
	}
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		return fmt.Errorf("read observability schema version: %w", err)
	}
	if version > SchemaVersion {
		return fmt.Errorf("observability schema %d is newer than supported version %d", version, SchemaVersion)
	}
	if version < 1 {
		if err := migrateV1(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at_ms) VALUES(?, ?)", 1, time.Now().UTC().UnixMilli()); err != nil {
			return fmt.Errorf("record observability schema version: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit observability migration: %w", err)
	}
	return nil
}

func migrateV1(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE host_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collected_at_ms INTEGER NOT NULL,
			cpu_percent REAL,
			memory_used_bytes INTEGER NOT NULL,
			memory_total_bytes INTEGER NOT NULL,
			load1 REAL NOT NULL,
			load5 REAL NOT NULL,
			load15 REAL NOT NULL,
			disk_used_bytes INTEGER NOT NULL,
			disk_total_bytes INTEGER NOT NULL,
			disk_read_bps REAL,
			disk_write_bps REAL,
			network_rx_bps REAL,
			network_tx_bps REAL,
			uptime_seconds REAL NOT NULL,
			boot_id TEXT NOT NULL
		)`,
		`CREATE INDEX host_samples_collected_at ON host_samples(collected_at_ms)`,
		`CREATE TABLE temperature_buckets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_start_ms INTEGER NOT NULL,
			bucket_end_ms INTEGER NOT NULL,
			sample_count INTEGER NOT NULL CHECK(sample_count > 0),
			min_celsius REAL NOT NULL,
			avg_celsius REAL NOT NULL,
			max_celsius REAL NOT NULL,
			peak_at_ms INTEGER NOT NULL
		)`,
		`CREATE INDEX temperature_buckets_start ON temperature_buckets(bucket_start_ms)`,
		`CREATE TABLE application_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collected_at_ms INTEGER NOT NULL,
			app_id TEXT NOT NULL,
			app_name TEXT NOT NULL,
			status TEXT NOT NULL,
			cpu_percent REAL NOT NULL,
			memory_used_bytes INTEGER NOT NULL,
			memory_limit_bytes INTEGER NOT NULL,
			network_rx_bps REAL,
			network_tx_bps REAL,
			restart_count INTEGER NOT NULL
		)`,
		`CREATE INDEX application_samples_collected_at ON application_samples(collected_at_ms)`,
		`CREATE INDEX application_samples_app_collected_at ON application_samples(app_id, collected_at_ms)`,
		`CREATE TABLE service_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collected_at_ms INTEGER NOT NULL,
			service_id TEXT NOT NULL,
			service_name TEXT NOT NULL,
			available INTEGER NOT NULL CHECK(available IN (0, 1)),
			status TEXT NOT NULL,
			restart_identity TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX service_samples_collected_at ON service_samples(collected_at_ms)`,
		`CREATE INDEX service_samples_service_collected_at ON service_samples(service_id, collected_at_ms)`,
		`CREATE TABLE events (
			event_id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			source_event_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			resource_name TEXT NOT NULL DEFAULT '',
			occurred_at_ms INTEGER NOT NULL,
			summary TEXT NOT NULL,
			details_json TEXT NOT NULL DEFAULT '{}',
			UNIQUE(source, source_event_id)
		)`,
		`CREATE INDEX events_occurred_at ON events(occurred_at_ms)`,
		`CREATE INDEX events_source_occurred_at ON events(source, occurred_at_ms)`,
		`CREATE TABLE collector_state (
			state_key TEXT PRIMARY KEY,
			state_value TEXT NOT NULL,
			updated_at_ms INTEGER NOT NULL
		)`,
		`CREATE TABLE alert_rules (
			rule_id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			metric TEXT NOT NULL,
			operator TEXT NOT NULL,
			threshold REAL NOT NULL,
			duration_seconds INTEGER NOT NULL CHECK(duration_seconds >= 0),
			enabled INTEGER NOT NULL CHECK(enabled IN (0, 1)),
			created_at_ms INTEGER NOT NULL,
			updated_at_ms INTEGER NOT NULL
		)`,
		`CREATE INDEX alert_rules_enabled ON alert_rules(enabled, scope, resource_id)`,
		`CREATE TABLE alert_incidents (
			incident_id TEXT PRIMARY KEY,
			rule_id TEXT NOT NULL REFERENCES alert_rules(rule_id),
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			opened_at_ms INTEGER NOT NULL,
			last_observed_at_ms INTEGER NOT NULL,
			resolved_at_ms INTEGER,
			state TEXT NOT NULL
		)`,
		`CREATE INDEX alert_incidents_rule_state ON alert_incidents(rule_id, state, opened_at_ms)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply observability schema v1: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) InsertBatch(ctx context.Context, batch Batch) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin observability batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, sample := range batch.Hosts {
		if sample.CollectedAt.IsZero() {
			return errors.New("host sample timestamp is required")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO host_samples (
			collected_at_ms, cpu_percent, memory_used_bytes, memory_total_bytes,
			load1, load5, load15, disk_used_bytes, disk_total_bytes,
			disk_read_bps, disk_write_bps, network_rx_bps, network_tx_bps,
			uptime_seconds, boot_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sample.CollectedAt.UTC().UnixMilli(), nullableFloat(sample.CPUPercent),
			sample.MemoryUsedBytes, sample.MemoryTotalBytes, sample.Load1, sample.Load5,
			sample.Load15, sample.DiskUsedBytes, sample.DiskTotalBytes,
			nullableFloat(sample.DiskReadBPS), nullableFloat(sample.DiskWriteBPS),
			nullableFloat(sample.NetworkRXBPS), nullableFloat(sample.NetworkTXBPS),
			sample.UptimeSeconds, sample.BootID)
		if err != nil {
			return fmt.Errorf("insert host sample: %w", err)
		}
	}
	for _, bucket := range batch.Temperatures {
		if bucket.SampleCount < 1 || bucket.BucketStart.IsZero() || bucket.PeakAt.IsZero() {
			return errors.New("invalid temperature bucket")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO temperature_buckets (
			bucket_start_ms, bucket_end_ms, sample_count, min_celsius,
			avg_celsius, max_celsius, peak_at_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?)`, bucket.BucketStart.UTC().UnixMilli(),
			bucket.BucketEnd.UTC().UnixMilli(), bucket.SampleCount, bucket.MinCelsius,
			bucket.AvgCelsius, bucket.MaxCelsius, bucket.PeakAt.UTC().UnixMilli())
		if err != nil {
			return fmt.Errorf("insert temperature bucket: %w", err)
		}
	}
	for _, sample := range batch.Applications {
		_, err = tx.ExecContext(ctx, `INSERT INTO application_samples (
			collected_at_ms, app_id, app_name, status, cpu_percent,
			memory_used_bytes, memory_limit_bytes, network_rx_bps,
			network_tx_bps, restart_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, sample.CollectedAt.UTC().UnixMilli(),
			sample.AppID, sample.Name, sample.Status, sample.CPUPercent,
			sample.MemoryUsedBytes, sample.MemoryLimitBytes,
			nullableFloat(sample.NetworkRXBPS), nullableFloat(sample.NetworkTXBPS),
			sample.RestartCount)
		if err != nil {
			return fmt.Errorf("insert application sample: %w", err)
		}
	}
	for _, sample := range batch.Services {
		_, err = tx.ExecContext(ctx, `INSERT INTO service_samples (
			collected_at_ms, service_id, service_name, available, status, restart_identity
		) VALUES (?, ?, ?, ?, ?, ?)`, sample.CollectedAt.UTC().UnixMilli(), sample.ServiceID,
			sample.Name, boolInt(sample.Available), sample.Status, sample.RestartIdentity)
		if err != nil {
			return fmt.Errorf("insert service sample: %w", err)
		}
	}
	for _, event := range batch.Events {
		if err := insertEvent(ctx, tx, event); err != nil {
			return err
		}
	}
	updatedAt := time.Now().UTC().UnixMilli()
	for key, value := range batch.State {
		_, err = tx.ExecContext(ctx, `INSERT INTO collector_state(state_key, state_value, updated_at_ms)
			VALUES(?, ?, ?) ON CONFLICT(state_key) DO UPDATE SET
			state_value=excluded.state_value, updated_at_ms=excluded.updated_at_ms`, key, value, updatedAt)
		if err != nil {
			return fmt.Errorf("update collector state: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit observability batch: %w", err)
	}
	return nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, event Event) error {
	if event.ID == "" || event.Source == "" || event.SourceEventID == "" || event.Type == "" || event.ResourceType == "" || event.OccurredAt.IsZero() || event.Summary == "" {
		return errors.New("invalid observability event")
	}
	details := []byte("{}")
	var err error
	if len(event.Details) > 0 {
		details, err = json.Marshal(event.Details)
		if err != nil {
			return fmt.Errorf("encode event details: %w", err)
		}
	}
	if len(details) > maxEventDetailsBytes {
		return errors.New("event details exceed safe size")
	}
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO events (
		event_id, source, source_event_id, event_type, resource_type,
		resource_id, resource_name, occurred_at_ms, summary, details_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.ID, event.Source,
		event.SourceEventID, event.Type, event.ResourceType, event.ResourceID,
		event.ResourceName, event.OccurredAt.UTC().UnixMilli(), event.Summary, string(details))
	if err != nil {
		return fmt.Errorf("insert observability event: %w", err)
	}
	return nil
}

func (s *Store) State(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT state_value FROM collector_state WHERE state_key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read collector state: %w", err)
	}
	return value, true, nil
}

func (s *Store) PruneMetrics(ctx context.Context, before time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin observability retention: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	cutoff := before.UTC().UnixMilli()
	for _, statement := range []string{
		"DELETE FROM host_samples WHERE collected_at_ms < ?",
		"DELETE FROM temperature_buckets WHERE bucket_start_ms < ?",
		"DELETE FROM application_samples WHERE collected_at_ms < ?",
		"DELETE FROM service_samples WHERE collected_at_ms < ?",
	} {
		if _, err := tx.ExecContext(ctx, statement, cutoff); err != nil {
			return fmt.Errorf("prune observability metrics: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit observability retention: %w", err)
	}
	return nil
}

func (s *Store) PutAlertRule(ctx context.Context, rule AlertRule) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO alert_rules (
		rule_id, scope, resource_id, metric, operator, threshold,
		duration_seconds, enabled, created_at_ms, updated_at_ms
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(rule_id) DO UPDATE SET scope=excluded.scope,
	resource_id=excluded.resource_id, metric=excluded.metric, operator=excluded.operator,
	threshold=excluded.threshold, duration_seconds=excluded.duration_seconds,
	enabled=excluded.enabled, updated_at_ms=excluded.updated_at_ms`, rule.ID, rule.Scope,
		rule.ResourceID, rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds,
		boolInt(rule.Enabled), rule.CreatedAt.UTC().UnixMilli(), rule.UpdatedAt.UTC().UnixMilli())
	if err != nil {
		return fmt.Errorf("store alert rule: %w", err)
	}
	return nil
}

func (s *Store) PutAlertIncident(ctx context.Context, incident AlertIncident) error {
	var resolved any
	if incident.ResolvedAt != nil {
		resolved = incident.ResolvedAt.UTC().UnixMilli()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO alert_incidents (
		incident_id, rule_id, resource_type, resource_id, opened_at_ms,
		last_observed_at_ms, resolved_at_ms, state
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(incident_id) DO UPDATE SET last_observed_at_ms=excluded.last_observed_at_ms,
	resolved_at_ms=excluded.resolved_at_ms, state=excluded.state`, incident.ID,
		incident.RuleID, incident.ResourceType, incident.ResourceID,
		incident.OpenedAt.UTC().UnixMilli(), incident.LastObservedAt.UTC().UnixMilli(),
		resolved, incident.State)
	if err != nil {
		return fmt.Errorf("store alert incident: %w", err)
	}
	return nil
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
