package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 1

type Store struct {
	db *sql.DB
}

type SystemSample struct {
	CollectedAt          time.Time
	CPUUsagePercent      float64
	Load1                float64
	Load5                float64
	Load15               float64
	MemoryUsedBytes      uint64
	MemoryUsagePercent   float64
	DiskUsedBytes        uint64
	DiskUsagePercent     float64
	TemperatureCelsius   float64
	UptimeSeconds        float64
	NetworkRXBytesPerSec float64
	NetworkTXBytesPerSec float64
}

type DeploymentSample struct {
	App             string
	Status          string
	ContainerCount  int
	CPUPercent      float64
	MemoryUsedBytes uint64
	RestartCount    int
	DatabaseState   string
}

type DatabaseSample struct {
	ID               string
	DisplayName      string
	Status           string
	SizeBytes        int64
	Connections      int64
	BackupCount      int
	BackupAgeSeconds *float64
	DeploymentState  string
}

type Batch struct {
	System      SystemSample
	Deployments []DeploymentSample
	Databases   []DatabaseSample
}

type ActivityEvent struct {
	OccurredAt  time.Time
	Source      string
	Kind        string
	Severity    string
	Subject     string
	Message     string
	Fingerprint string
}

type Counts struct {
	System      int
	Deployments int
	Databases   int
	Activity    int
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("history database path is required")
	}
	if err := ensureParent(path); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open history database: %w", err)
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
	return store, nil
}

func ensureParent(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return fmt.Errorf("create history database directory: %w", err)
	}
	return nil
}

func (s *Store) configure(ctx context.Context) error {
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure history database: %w", err)
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at INTEGER NOT NULL
)`); err != nil {
		return fmt.Errorf("create history schema migrations: %w", err)
	}

	var version int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return fmt.Errorf("read history schema version: %w", err)
	}
	if version > SchemaVersion {
		return fmt.Errorf("history database schema %d is newer than supported version %d", version, SchemaVersion)
	}

	if version < 1 {
		if err := migrateV1(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)",
			1,
			time.Now().UTC().Unix(),
		); err != nil {
			return fmt.Errorf("record history schema version: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history migration: %w", err)
	}
	return nil
}

func migrateV1(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE system_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collected_at INTEGER NOT NULL,
			cpu_usage_percent REAL NOT NULL,
			load1 REAL NOT NULL,
			load5 REAL NOT NULL,
			load15 REAL NOT NULL,
			memory_used_bytes INTEGER NOT NULL,
			memory_usage_percent REAL NOT NULL,
			disk_used_bytes INTEGER NOT NULL,
			disk_usage_percent REAL NOT NULL,
			temperature_celsius REAL NOT NULL,
			uptime_seconds REAL NOT NULL,
			network_rx_bytes_per_sec REAL NOT NULL,
			network_tx_bytes_per_sec REAL NOT NULL
		)`,
		`CREATE INDEX system_samples_collected_at ON system_samples(collected_at)`,
		`CREATE TABLE deployment_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collected_at INTEGER NOT NULL,
			app TEXT NOT NULL,
			status TEXT NOT NULL,
			container_count INTEGER NOT NULL,
			cpu_percent REAL NOT NULL,
			memory_used_bytes INTEGER NOT NULL,
			restart_count INTEGER NOT NULL,
			database_state TEXT NOT NULL
		)`,
		`CREATE INDEX deployment_samples_collected_at ON deployment_samples(collected_at)`,
		`CREATE INDEX deployment_samples_app_collected_at ON deployment_samples(app, collected_at)`,
		`CREATE TABLE database_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collected_at INTEGER NOT NULL,
			database_id TEXT NOT NULL,
			display_name TEXT NOT NULL,
			status TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			connections INTEGER NOT NULL,
			backup_count INTEGER NOT NULL,
			backup_age_seconds REAL,
			deployment_state TEXT NOT NULL
		)`,
		`CREATE INDEX database_samples_collected_at ON database_samples(collected_at)`,
		`CREATE INDEX database_samples_database_collected_at ON database_samples(database_id, collected_at)`,
		`CREATE TABLE activity_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at INTEGER NOT NULL,
			source TEXT NOT NULL,
			kind TEXT NOT NULL,
			severity TEXT NOT NULL,
			subject TEXT NOT NULL,
			message TEXT NOT NULL,
			fingerprint TEXT NOT NULL
		)`,
		`CREATE INDEX activity_events_occurred_at ON activity_events(occurred_at)`,
		`CREATE INDEX activity_events_source_occurred_at ON activity_events(source, occurred_at)`,
	}

	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply history schema v1: %w", err)
		}
	}
	return nil
}

func (s *Store) InsertBatch(ctx context.Context, batch Batch) error {
	if batch.System.CollectedAt.IsZero() {
		return errors.New("system sample collected time is required")
	}
	collectedAt := batch.System.CollectedAt.UTC().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history sample batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO system_samples (
	collected_at,
	cpu_usage_percent,
	load1,
	load5,
	load15,
	memory_used_bytes,
	memory_usage_percent,
	disk_used_bytes,
	disk_usage_percent,
	temperature_celsius,
	uptime_seconds,
	network_rx_bytes_per_sec,
	network_tx_bytes_per_sec
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		collectedAt,
		batch.System.CPUUsagePercent,
		batch.System.Load1,
		batch.System.Load5,
		batch.System.Load15,
		batch.System.MemoryUsedBytes,
		batch.System.MemoryUsagePercent,
		batch.System.DiskUsedBytes,
		batch.System.DiskUsagePercent,
		batch.System.TemperatureCelsius,
		batch.System.UptimeSeconds,
		batch.System.NetworkRXBytesPerSec,
		batch.System.NetworkTXBytesPerSec,
	); err != nil {
		return fmt.Errorf("insert system history sample: %w", err)
	}

	for _, sample := range batch.Deployments {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO deployment_samples (
	collected_at,
	app,
	status,
	container_count,
	cpu_percent,
	memory_used_bytes,
	restart_count,
	database_state
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			collectedAt,
			sample.App,
			sample.Status,
			sample.ContainerCount,
			sample.CPUPercent,
			sample.MemoryUsedBytes,
			sample.RestartCount,
			sample.DatabaseState,
		); err != nil {
			return fmt.Errorf("insert deployment history sample: %w", err)
		}
	}

	for _, sample := range batch.Databases {
		var backupAge any
		if sample.BackupAgeSeconds != nil {
			backupAge = *sample.BackupAgeSeconds
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO database_samples (
	collected_at,
	database_id,
	display_name,
	status,
	size_bytes,
	connections,
	backup_count,
	backup_age_seconds,
	deployment_state
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			collectedAt,
			sample.ID,
			sample.DisplayName,
			sample.Status,
			sample.SizeBytes,
			sample.Connections,
			sample.BackupCount,
			backupAge,
			sample.DeploymentState,
		); err != nil {
			return fmt.Errorf("insert database history sample: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history sample batch: %w", err)
	}
	return nil
}

func (s *Store) AppendActivity(ctx context.Context, event ActivityEvent) error {
	if event.OccurredAt.IsZero() {
		return errors.New("activity occurrence time is required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO activity_events (
	occurred_at,
	source,
	kind,
	severity,
	subject,
	message,
	fingerprint
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.OccurredAt.UTC().Unix(),
		event.Source,
		event.Kind,
		event.Severity,
		event.Subject,
		event.Message,
		event.Fingerprint,
	)
	if err != nil {
		return fmt.Errorf("insert activity event: %w", err)
	}
	return nil
}

func (s *Store) PruneBefore(ctx context.Context, cutoff time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history prune: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	cutoffUnix := cutoff.UTC().Unix()
	for _, statement := range []string{
		"DELETE FROM system_samples WHERE collected_at < ?",
		"DELETE FROM deployment_samples WHERE collected_at < ?",
		"DELETE FROM database_samples WHERE collected_at < ?",
		"DELETE FROM activity_events WHERE occurred_at < ?",
	} {
		if _, err := tx.ExecContext(ctx, statement, cutoffUnix); err != nil {
			return fmt.Errorf("prune history data: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history prune: %w", err)
	}
	return nil
}

func (s *Store) Counts(ctx context.Context) (Counts, error) {
	var counts Counts
	queries := []struct {
		query string
		dest  *int
	}{
		{"SELECT COUNT(*) FROM system_samples", &counts.System},
		{"SELECT COUNT(*) FROM deployment_samples", &counts.Deployments},
		{"SELECT COUNT(*) FROM database_samples", &counts.Databases},
		{"SELECT COUNT(*) FROM activity_events", &counts.Activity},
	}
	for _, item := range queries {
		if err := s.db.QueryRowContext(ctx, item.query).Scan(item.dest); err != nil {
			return Counts{}, fmt.Errorf("count history rows: %w", err)
		}
	}
	return counts, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
