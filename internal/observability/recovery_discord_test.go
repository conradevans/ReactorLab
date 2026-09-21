package observability

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/system"
)

func recoveryDiscordFixture(suffix string) (Event, RecoveryIncident) {
	lastKnownAlive := time.Date(2026, 9, 21, 8, 0, 0, 0, time.FixedZone("UTC", 0)).
		Add(time.Duration(len(suffix)) * time.Minute)
	recoveredAt := lastKnownAlive.Add(3*time.Hour + 2*time.Minute + 4*time.Second)
	event := newEvent(
		"reactorlab",
		"recovery:boot-"+suffix,
		recoveryEventType,
		"host",
		"dell",
		"Dell host",
		recoveredAt,
		"Dell host recovered after an unexpected shutdown",
		map[string]any{
			"lastKnownAliveAt": lastKnownAlive,
			"recoveredAt":      recoveredAt,
			"downtimeSeconds":  int64(recoveredAt.Sub(lastKnownAlive) / time.Second),
			"status":           recoveryIncidentStatus,
			"previousBootId":   "previous-" + suffix,
			"recoveryBootId":   "recovery-" + suffix,
		},
	)
	return event, RecoveryIncident{
		EventID:          event.ID,
		LastKnownAliveAt: lastKnownAlive,
		RecoveredAt:      recoveredAt,
		DowntimeSeconds:  int64(recoveredAt.Sub(lastKnownAlive) / time.Second),
		Status:           recoveryIncidentStatus,
		PreviousBootID:   "previous-" + suffix,
		RecoveryBootID:   "recovery-" + suffix,
	}
}

func insertRecoveryDiscordFixture(
	t *testing.T,
	store *Store,
	suffix string,
	notify bool,
) RecoveryIncident {
	t.Helper()
	event, incident := recoveryDiscordFixture(suffix)
	if err := store.InsertBatch(context.Background(), Batch{
		Recoveries: []RecoveryIncidentWrite{{Event: event, Notify: notify}},
	}); err != nil {
		t.Fatal(err)
	}
	return incident
}

type recoveryDiscordOutboxRow struct {
	state         string
	attemptCount  int
	nextAttemptAt time.Time
	sentAt        sql.NullInt64
	lastError     string
}

func readRecoveryDiscordOutboxRow(
	t *testing.T,
	store *Store,
	eventID string,
) (recoveryDiscordOutboxRow, bool) {
	t.Helper()
	var row recoveryDiscordOutboxRow
	var nextAttemptAt int64
	err := store.db.QueryRow(
		`SELECT state, attempt_count, next_attempt_at_ms, sent_at_ms, last_error
		 FROM recovery_discord_outbox WHERE event_id = ?`,
		eventID,
	).Scan(
		&row.state,
		&row.attemptCount,
		&nextAttemptAt,
		&row.sentAt,
		&row.lastError,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return recoveryDiscordOutboxRow{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	row.nextAttemptAt = time.UnixMilli(nextAttemptAt).UTC()
	return row, true
}

func TestOpenMigratesV2ToV3AndPreservesExistingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observability", "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	event := newEvent(
		"test",
		"preserved",
		"host_restart",
		"host",
		"dell",
		"Dell host",
		time.Date(2026, 9, 21, 7, 0, 0, 0, time.UTC),
		"preserved event",
		nil,
	)
	if err := store.InsertBatch(ctx, Batch{
		Hosts: []HostSample{{
			CollectedAt:      event.OccurredAt,
			MemoryTotalBytes: 1,
			DiskTotalBytes:   1,
			BootID:           "preserved-boot",
		}},
		Events: []Event{event},
		State: map[string]string{
			hostSessionStateKey: "preserved-session",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "DROP TABLE recovery_discord_outbox"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(
		ctx,
		"DELETE FROM schema_migrations WHERE version = 3",
	); err != nil {
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

	var version, eventCount int
	if err := migrated.db.QueryRow(
		"SELECT MAX(version) FROM schema_migrations",
	).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 3 {
		t.Fatalf("schema version = %d, want 3", version)
	}
	outboxTables, err := migrated.db.Query(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'recovery_%_outbox' ORDER BY name",
	)
	if err != nil {
		t.Fatal(err)
	}
	var tableNames []string
	for outboxTables.Next() {
		var name string
		if err := outboxTables.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tableNames = append(tableNames, name)
	}
	if err := outboxTables.Close(); err != nil {
		t.Fatal(err)
	}
	if len(tableNames) != 1 || tableNames[0] != "recovery_discord_outbox" {
		t.Fatalf("recovery outbox tables = %v", tableNames)
	}
	if err := migrated.db.QueryRow(
		"SELECT COUNT(*) FROM events WHERE event_id = ?",
		event.ID,
	).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("preserved event count = %d, want 1", eventCount)
	}
	value, ok, err := migrated.State(ctx, hostSessionStateKey)
	if err != nil || !ok || value != "preserved-session" {
		t.Fatalf("preserved session = %q, found=%t, err=%v", value, ok, err)
	}
	var hostCount int
	if err := migrated.db.QueryRow(
		"SELECT COUNT(*) FROM host_samples WHERE boot_id = ?",
		"preserved-boot",
	).Scan(&hostCount); err != nil {
		t.Fatal(err)
	}
	if hostCount != 1 {
		t.Fatalf("preserved host samples = %d, want 1", hostCount)
	}

	rows, err := migrated.db.Query("PRAGMA index_info(recovery_discord_outbox_due)")
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
	wantColumns := []string{"state", "next_attempt_at_ms", "created_at_ms"}
	if strings.Join(columns, ",") != strings.Join(wantColumns, ",") {
		t.Fatalf("outbox index columns = %v, want %v", columns, wantColumns)
	}

	if err := migrated.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryIncidentNotificationIntentAndDeduplication(t *testing.T) {
	t.Run("disabled live write has no outbox item", func(t *testing.T) {
		store := openTestStore(t)
		incident := insertRecoveryDiscordFixture(t, store, "disabled", false)
		if _, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID); found {
			t.Fatal("disabled incident unexpectedly queued an Discord notification")
		}
	})

	t.Run("enabled live write queues exactly one item", func(t *testing.T) {
		store := openTestStore(t)
		event, incident := recoveryDiscordFixture("enabled")
		batch := Batch{Recoveries: []RecoveryIncidentWrite{{
			Event: event, Notify: true,
		}}}
		if err := store.InsertBatch(context.Background(), batch); err != nil {
			t.Fatal(err)
		}
		if err := store.InsertBatch(context.Background(), batch); err != nil {
			t.Fatal(err)
		}
		row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
		if !found || row.state != recoveryDiscordStatePending ||
			row.attemptCount != 0 || row.sentAt.Valid {
			t.Fatalf("outbox row = %#v, found=%t", row, found)
		}
		var eventCount, outboxCount int
		if err := store.db.QueryRow(
			"SELECT COUNT(*) FROM events WHERE event_id = ?",
			incident.EventID,
		).Scan(&eventCount); err != nil {
			t.Fatal(err)
		}
		if err := store.db.QueryRow(
			"SELECT COUNT(*) FROM recovery_discord_outbox WHERE event_id = ?",
			incident.EventID,
		).Scan(&outboxCount); err != nil {
			t.Fatal(err)
		}
		if eventCount != 1 || outboxCount != 1 {
			t.Fatalf("event count=%d outbox count=%d", eventCount, outboxCount)
		}
	})

	t.Run("historical notify false never becomes stale work", func(t *testing.T) {
		store := openTestStore(t)
		event, incident := recoveryDiscordFixture("historical")
		if err := store.InsertBatch(context.Background(), Batch{
			Recoveries: []RecoveryIncidentWrite{{Event: event, Notify: false}},
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.InsertBatch(context.Background(), Batch{
			Recoveries: []RecoveryIncidentWrite{{Event: event, Notify: true}},
		}); err != nil {
			t.Fatal(err)
		}
		if _, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID); found {
			t.Fatal("historical incident unexpectedly queued a Discord notification")
		}
	})

	t.Run("event outbox host sample and session state roll back together", func(t *testing.T) {
		store := openTestStore(t)
		event, incident := recoveryDiscordFixture("rollback")
		err := store.InsertBatch(context.Background(), Batch{
			Hosts: []HostSample{{
				CollectedAt:      event.OccurredAt,
				MemoryTotalBytes: 1,
				DiskTotalBytes:   1,
				BootID:           "rollback-boot",
			}},
			Recoveries: []RecoveryIncidentWrite{
				{Event: event, Notify: true},
				{Event: Event{}, Notify: true},
			},
			State: map[string]string{hostSessionStateKey: "must-not-commit"},
		})
		if err == nil {
			t.Fatal("invalid transaction unexpectedly committed")
		}
		var eventCount int
		if err := store.db.QueryRow(
			"SELECT COUNT(*) FROM events WHERE event_id = ?",
			incident.EventID,
		).Scan(&eventCount); err != nil {
			t.Fatal(err)
		}
		if eventCount != 0 {
			t.Fatalf("recovery event count = %d, want 0", eventCount)
		}
		if _, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID); found {
			t.Fatal("rolled-back outbox item remains")
		}
		if _, found, err := store.State(
			context.Background(),
			hostSessionStateKey,
		); err != nil || found {
			t.Fatalf("rolled-back session found=%t err=%v", found, err)
		}
		var hostCount int
		if err := store.db.QueryRow(
			"SELECT COUNT(*) FROM host_samples WHERE boot_id = ?",
			"rollback-boot",
		).Scan(&hostCount); err != nil {
			t.Fatal(err)
		}
		if hostCount != 0 {
			t.Fatalf("rolled-back host sample count = %d, want 0", hostCount)
		}
	})
}

func TestCollectorLiveRecoveryUsesConfiguredNotificationIntent(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled bool
		wantRow bool
	}{
		{name: "disabled", enabled: false, wantRow: false},
		{name: "enabled", enabled: true, wantRow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := openTestStore(t)
			collector := NewCollector(store, CollectorConfig{
				RecoveryDiscordEnabled: test.enabled,
			})
			previous := HostSession{
				BootID:        "boot-before-" + test.name,
				BootedAt:      time.Date(2026, 9, 21, 6, 0, 0, 0, time.UTC),
				LastSeenAt:    time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
				CleanShutdown: false,
			}
			if err := collector.sessionTracker.SetBaseline(
				encodedHostSession(t, previous),
			); err != nil {
				t.Fatal(err)
			}
			observedAt := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
			collector.now = func() time.Time { return observedAt }
			collector.readHost = func(time.Time) (system.HistoricalSnapshot, error) {
				return system.HistoricalSnapshot{
					ObservedAt:    observedAt,
					UptimeSeconds: 3600,
					BootID:        "boot-after-" + test.name,
				}, nil
			}
			collector.intervals.host = time.Hour

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				collector.hostLoop(ctx)
			}()
			var message writerMessage
			select {
			case message = <-collector.criticalQueue:
			case <-time.After(time.Second):
				cancel()
				t.Fatal("live recovery was not queued for persistence")
			}
			cancel()
			<-done
			if len(message.batch.Recoveries) != 1 ||
				message.batch.Recoveries[0].Notify != test.enabled {
				t.Fatalf("recovery writes = %#v", message.batch.Recoveries)
			}
			if err := store.InsertBatch(context.Background(), message.batch); err != nil {
				t.Fatal(err)
			}
			eventID := message.batch.Recoveries[0].Event.ID
			if _, found := readRecoveryDiscordOutboxRow(t, store, eventID); found != test.wantRow {
				t.Fatalf("outbox found=%t, want %t", found, test.wantRow)
			}
			incident, err := NewQueryService(store).RecoveryIncidentByID(
				context.Background(),
				eventID,
			)
			if err != nil || incident == nil {
				t.Fatalf("incident=%#v err=%v", incident, err)
			}
		})
	}
}

func TestRecoveryDiscordOutboxUniquenessAndRetention(t *testing.T) {
	store := openTestStore(t)
	incident := insertRecoveryDiscordFixture(t, store, "unique", true)
	row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
	if !found {
		t.Fatal("outbox item was not created")
	}
	if _, err := store.db.Exec(
		`INSERT INTO recovery_discord_outbox (
			event_id, state, attempt_count, next_attempt_at_ms, sent_at_ms,
			last_error, created_at_ms
		) VALUES (?, 'pending', 0, ?, NULL, '', ?)`,
		incident.EventID,
		row.nextAttemptAt.UnixMilli(),
		row.nextAttemptAt.UnixMilli(),
	); err == nil {
		t.Fatal("duplicate event_id was accepted")
	}
	if err := store.PruneMetrics(
		context.Background(),
		time.Now().UTC().Add(time.Hour),
	); err != nil {
		t.Fatal(err)
	}
	if _, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID); !found {
		t.Fatal("metrics retention removed recovery outbox work")
	}
}

func recoveryDiscordLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func completeRecoveryDiscordEnvironment(webhookFile string) map[string]string {
	return map[string]string{
		"REACTORLAB_RECOVERY_DISCORD_ENABLED":       "true",
		"REACTORLAB_DISCORD_WEBHOOK_FILE":           webhookFile,
		"REACTORLAB_DISCORD_USERNAME":               "ReactorLab",
		"REACTORLAB_RECOVERY_NOTIFICATION_TIMEZONE": "America/New_York",
	}
}

func TestRecoveryDiscordConfiguration(t *testing.T) {
	t.Run("disabled and unset", func(t *testing.T) {
		for _, values := range []map[string]string{
			{},
			{"REACTORLAB_RECOVERY_DISCORD_ENABLED": "false"},
		} {
			config, err := recoveryDiscordConfigFromLookup(recoveryDiscordLookup(values))
			if err != nil || config.Enabled {
				t.Fatalf("config=%#v err=%v", config, err)
			}
			if config.WebhookFile != defaultDiscordWebhookFile ||
				config.Location != time.UTC || config.Timezone != "UTC" {
				t.Fatalf("defaults = %#v", config)
			}
		}
	})

	t.Run("accepted boolean forms and complete settings", func(t *testing.T) {
		for _, value := range []string{"true", "TRUE", "1", "t"} {
			values := completeRecoveryDiscordEnvironment("/etc/reactorlab/discord-webhook")
			values["REACTORLAB_RECOVERY_DISCORD_ENABLED"] = value
			config, err := recoveryDiscordConfigFromLookup(recoveryDiscordLookup(values))
			if err != nil || !config.Enabled || config.Username != "ReactorLab" ||
				config.Location.String() != "America/New_York" || config.TimezoneFallback {
				t.Fatalf("value=%q config=%#v err=%v", value, config, err)
			}
			if _, err := NewDiscordWebhookSender(config); err != nil {
				t.Fatalf("sender error=%v", err)
			}
		}
	})

	t.Run("enabled defaults webhook path", func(t *testing.T) {
		config, err := recoveryDiscordConfigFromLookup(recoveryDiscordLookup(
			map[string]string{"REACTORLAB_RECOVERY_DISCORD_ENABLED": "true"},
		))
		if err != nil || !config.Enabled || config.WebhookFile != defaultDiscordWebhookFile {
			t.Fatalf("config=%#v err=%v", config, err)
		}
	})

	t.Run("malformed flag and relative secret path rejected", func(t *testing.T) {
		for name, values := range map[string]map[string]string{
			"flag": {
				"REACTORLAB_RECOVERY_DISCORD_ENABLED": "sometimes",
			},
			"relative path": {
				"REACTORLAB_RECOVERY_DISCORD_ENABLED": "true",
				"REACTORLAB_DISCORD_WEBHOOK_FILE":     "discord-webhook",
			},
			"control in username": {
				"REACTORLAB_RECOVERY_DISCORD_ENABLED": "true",
				"REACTORLAB_DISCORD_WEBHOOK_FILE":     "/secure/discord-webhook",
				"REACTORLAB_DISCORD_USERNAME":         "unsafe\nname",
			},
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := recoveryDiscordConfigFromLookup(
					recoveryDiscordLookup(values),
				); err == nil {
					t.Fatal("malformed configuration was accepted")
				}
			})
		}
	})

	t.Run("invalid timezone degrades to UTC", func(t *testing.T) {
		values := completeRecoveryDiscordEnvironment("/etc/reactorlab/discord-webhook")
		values["REACTORLAB_RECOVERY_NOTIFICATION_TIMEZONE"] = "Mars/Olympus_Mons"
		config, err := recoveryDiscordConfigFromLookup(recoveryDiscordLookup(values))
		if err != nil || config.Location != time.UTC || config.Timezone != "UTC" ||
			!config.TimezoneFallback {
			t.Fatalf("config=%#v err=%v", config, err)
		}
	})

	t.Run("disabled ignores incomplete delivery settings", func(t *testing.T) {
		config, err := recoveryDiscordConfigFromLookup(recoveryDiscordLookup(
			map[string]string{
				"REACTORLAB_RECOVERY_DISCORD_ENABLED": "false",
				"REACTORLAB_DISCORD_WEBHOOK_FILE":     "relative",
			},
		))
		if err != nil || config.Enabled {
			t.Fatalf("config=%#v err=%v", config, err)
		}
	})
}

func writeDiscordWebhookSecret(t *testing.T, value string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "discord-webhook")
	if err := os.WriteFile(path, []byte(value+"\n"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscordWebhookFileSecurityAndErrorsDoNotExposeSecret(t *testing.T) {
	secret := "https://example.com/api/webhooks/123/test-token-never-log"
	path := writeDiscordWebhookSecret(t, secret, 0o600)
	webhookURL, err := readDiscordWebhookURL(path)
	if err != nil || webhookURL != secret {
		t.Fatalf("webhook read failed: len=%d err=%v", len(webhookURL), err)
	}

	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := readDiscordWebhookURL(path); err != nil {
		t.Fatalf("group-readable webhook rejected: %v", err)
	}

	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := readDiscordWebhookURL(path); !errors.Is(
		err,
		ErrRecoveryDiscordDeliveryUnavailable,
	) {
		t.Fatalf("unreadable webhook file err=%v", err)
	}
	for _, mode := range []os.FileMode{0o644, 0o660, 0o700} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		_, err := readDiscordWebhookURL(path)
		if !errors.Is(err, ErrRecoveryDiscordDeliveryUnavailable) {
			t.Fatalf("mode %o err=%v", mode, err)
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), path) {
			t.Fatalf("webhook-file error exposed sensitive data: %q", err)
		}
	}

	if _, err := readDiscordWebhookURL(filepath.Join(t.TempDir(), "missing")); !errors.Is(
		err,
		ErrRecoveryDiscordDeliveryUnavailable,
	) {
		t.Fatalf("missing webhook file err=%v", err)
	}
	if _, err := readDiscordWebhookURL(t.TempDir()); !errors.Is(
		err,
		ErrRecoveryDiscordDeliveryUnavailable,
	) {
		t.Fatalf("non-regular webhook file err=%v", err)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(t.TempDir(), "discord-webhook-link")
	if err := os.Symlink(path, linkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := readDiscordWebhookURL(linkPath); !errors.Is(
		err,
		ErrRecoveryDiscordDeliveryUnavailable,
	) {
		t.Fatalf("symlink webhook file err=%v", err)
	}
}

func TestDiscordWebhookURLValidation(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid", value: "https://discord.example/api/webhooks/123/token", valid: true},
		{name: "HTTP rejected", value: "http://discord.example/api/webhooks/123/token"},
		{name: "non-HTTP rejected", value: "file:///api/webhooks/123/token"},
		{name: "missing host", value: "https:///api/webhooks/123/token"},
		{name: "malformed", value: "://not-a-url"},
		{name: "wrong path", value: "https://discord.example/not-a-webhook"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateDiscordWebhookURL(test.value)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t err=%v", test.valid, err)
			}
			if err != nil && strings.Contains(err.Error(), test.value) {
				t.Fatalf("validation error exposed webhook URL: %v", err)
			}
		})
	}
}

func newTestDiscordWebhookSender(
	t *testing.T,
	handler http.HandlerFunc,
) (*DiscordWebhookSender, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	path := writeDiscordWebhookSecret(
		t,
		server.URL+"/api/webhooks/123/test-token-never-log",
		0o600,
	)
	sender, err := NewDiscordWebhookSender(RecoveryDiscordConfig{
		Enabled: true, WebhookFile: path, Username: "ReactorLab",
	})
	if err != nil {
		t.Fatal(err)
	}
	sender.client = server.Client()
	return sender, server
}

func TestDiscordWebhookSenderSuccessPayload(t *testing.T) {
	requestSeen := make(chan struct{}, 1)
	sender, _ := newTestDiscordWebhookSender(t, func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("method=%s content-type=%q", request.Method, request.Header.Get("Content-Type"))
		}
		var payload struct {
			Content  string `json:"content"`
			Username string `json:"username"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.Content != "recovered" || payload.Username != "ReactorLab" {
			t.Errorf("payload=%#v", payload)
		}
		requestSeen <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := sender.SendRecoveryDiscord(
		context.Background(),
		RecoveryDiscordMessage{Content: "recovered"},
	); err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestSeen:
	case <-time.After(time.Second):
		t.Fatal("webhook request was not observed")
	}
}

func TestDiscordWebhookSenderHTTPFailuresAreSanitized(t *testing.T) {
	secretBody := "provider-response-secret-never-persist"
	for _, test := range []struct {
		name       string
		status     int
		retryAfter string
		category   string
		wantDelay  time.Duration
	}{
		{name: "rate limited", status: 429, retryAfter: "600", category: discordErrorRateLimited, wantDelay: 10 * time.Minute},
		{name: "server error", status: 500, category: discordErrorServer},
		{name: "forbidden", status: 403, category: discordErrorForbidden},
		{name: "not found", status: 404, category: discordErrorWebhookNotFound},
		{name: "unauthorized", status: 401, category: discordErrorInvalidConfiguration},
	} {
		t.Run(test.name, func(t *testing.T) {
			sender, _ := newTestDiscordWebhookSender(t, func(w http.ResponseWriter, _ *http.Request) {
				if test.retryAfter != "" {
					w.Header().Set("Retry-After", test.retryAfter)
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(secretBody))
			})
			err := sender.SendRecoveryDiscord(
				context.Background(),
				RecoveryDiscordMessage{Content: "recovered"},
			)
			category, delay := recoveryDiscordFailureDetails(err)
			if category != test.category || delay != test.wantDelay {
				t.Fatalf("category=%q delay=%s err=%v", category, delay, err)
			}
			if strings.Contains(err.Error(), secretBody) ||
				strings.Contains(err.Error(), "test-token-never-log") {
				t.Fatalf("error exposed provider or webhook secret: %v", err)
			}
		})
	}
}

func TestDiscordWebhookSenderNetworkTimeoutIsRetryable(t *testing.T) {
	sender, _ := newTestDiscordWebhookSender(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	})
	sender.client.Timeout = 10 * time.Millisecond
	err := sender.SendRecoveryDiscord(
		context.Background(),
		RecoveryDiscordMessage{Content: "recovered"},
	)
	category, _ := recoveryDiscordFailureDetails(err)
	if category != discordErrorNetwork {
		t.Fatalf("category=%q err=%v", category, err)
	}
}

type fakeRecoveryDiscordSender struct {
	mu       sync.Mutex
	calls    int
	messages []RecoveryDiscordMessage
	err      error
}

func (s *fakeRecoveryDiscordSender) SendRecoveryDiscord(
	_ context.Context,
	message RecoveryDiscordMessage,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.messages = append(s.messages, message)
	return s.err
}

func (s *fakeRecoveryDiscordSender) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func newRecoveryDiscordWorkerForTest(
	store *Store,
	sender RecoveryDiscordSender,
	now time.Time,
) *RecoveryDiscordWorker {
	worker := NewRecoveryDiscordWorker(store, sender, time.UTC)
	worker.now = func() time.Time { return now }
	worker.pollInterval = 2 * time.Millisecond
	worker.errors = newErrorReporter(0)
	worker.inspectProtection = func(context.Context) system.RecoveryProtectionState {
		return system.RecoveryProtectionState{
			State: system.RecoveryProtectionArmed,
			HardwareWatchdog: system.HardwareWatchdogProtectionState{
				State: system.RecoveryProtectionArmed,
			},
			RTC: system.RTCRecoveryProtectionState{
				State: system.RecoveryProtectionArmed,
			},
		}
	}
	return worker
}

func TestRecoveryDiscordWorkerSuccessDeduplicationAndRestart(t *testing.T) {
	store := openTestStore(t)
	incident := insertRecoveryDiscordFixture(t, store, "success", true)
	now := time.Now().UTC().Add(time.Hour)
	sender := &fakeRecoveryDiscordSender{}
	worker := newRecoveryDiscordWorkerForTest(store, sender, now)
	if !worker.processOne(context.Background()) {
		t.Fatal("pending outbox item was not processed")
	}
	row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
	if !found || row.state != recoveryDiscordStateSent || row.attemptCount != 1 ||
		!row.sentAt.Valid || row.lastError != "" {
		t.Fatalf("sent row = %#v, found=%t", row, found)
	}
	if sender.callCount() != 1 {
		t.Fatalf("send calls = %d, want 1", sender.callCount())
	}
	if worker.processOne(context.Background()) {
		t.Fatal("sent item was processed again")
	}

	restartedSender := &fakeRecoveryDiscordSender{}
	restarted := newRecoveryDiscordWorkerForTest(store, restartedSender, now.Add(time.Hour))
	if restarted.processOne(context.Background()) {
		t.Fatal("restarted worker resent a sent item")
	}
	if restartedSender.callCount() != 0 {
		t.Fatalf("restart send calls = %d, want 0", restartedSender.callCount())
	}
}

func TestRecoveryDiscordWorkerFailureRetryAndFutureScheduling(t *testing.T) {
	store := openTestStore(t)
	incident := insertRecoveryDiscordFixture(t, store, "retry", true)
	now := time.Now().UTC().Add(time.Hour)
	secret := "provider-error-containing-a-secret"
	failingSender := &fakeRecoveryDiscordSender{err: errors.New(secret)}

	var logs bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(originalLogOutput) })

	worker := newRecoveryDiscordWorkerForTest(store, failingSender, now)
	if !worker.processOne(context.Background()) {
		t.Fatal("pending item was not attempted")
	}
	row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
	if !found || row.state != recoveryDiscordStateRetry || row.attemptCount != 1 {
		t.Fatalf("retry row = %#v, found=%t", row, found)
	}
	wantNextAttempt := time.UnixMilli(now.Add(time.Minute).UnixMilli()).UTC()
	if !row.nextAttemptAt.Equal(wantNextAttempt) {
		t.Fatalf("next attempt = %s, want %s", row.nextAttemptAt, wantNextAttempt)
	}
	if row.lastError != discordErrorDeliveryFailed ||
		strings.Contains(row.lastError, secret) ||
		strings.Contains(logs.String(), secret) {
		t.Fatalf("unsafe persisted/logged error: row=%q logs=%q", row.lastError, logs.String())
	}

	successSender := &fakeRecoveryDiscordSender{}
	early := newRecoveryDiscordWorkerForTest(
		store,
		successSender,
		now.Add(30*time.Second),
	)
	if early.processOne(context.Background()) {
		t.Fatal("future retry was processed early")
	}
	if successSender.callCount() != 0 {
		t.Fatalf("early send calls = %d, want 0", successSender.callCount())
	}

	restarted := newRecoveryDiscordWorkerForTest(
		store,
		successSender,
		now.Add(time.Minute),
	)
	if !restarted.processOne(context.Background()) {
		t.Fatal("due retry was not picked up after worker recreation")
	}
	if successSender.callCount() != 1 {
		t.Fatalf("retry send calls = %d, want 1", successSender.callCount())
	}
	row, _ = readRecoveryDiscordOutboxRow(t, store, incident.EventID)
	if row.state != recoveryDiscordStateSent || row.attemptCount != 2 {
		t.Fatalf("retried row = %#v", row)
	}
}

func TestRecoveryDiscordWorkerHonorsRetryAfter(t *testing.T) {
	store := openTestStore(t)
	incident := insertRecoveryDiscordFixture(t, store, "rate-limit", true)
	now := time.Now().UTC().Add(time.Hour)
	sender := &fakeRecoveryDiscordSender{
		err: newDiscordDeliveryError(discordErrorRateLimited, 10*time.Minute),
	}
	worker := newRecoveryDiscordWorkerForTest(store, sender, now)
	if !worker.processOne(context.Background()) {
		t.Fatal("rate-limited item was not attempted")
	}
	row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
	wantNextAttempt := time.UnixMilli(now.Add(10 * time.Minute).UnixMilli()).UTC()
	if !found || row.state != recoveryDiscordStateRetry || row.attemptCount != 1 ||
		row.lastError != discordErrorRateLimited || !row.nextAttemptAt.Equal(wantNextAttempt) {
		t.Fatalf("rate-limit row=%#v found=%t wantNext=%s", row, found, wantNextAttempt)
	}
}

func TestRecoveryDiscordWorkerDoesNotPersistResponseBody(t *testing.T) {
	secretBody := "provider-response-secret-never-persist"
	sender, _ := newTestDiscordWebhookSender(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(secretBody))
	})
	store := openTestStore(t)
	incident := insertRecoveryDiscordFixture(t, store, "response-body", true)
	worker := newRecoveryDiscordWorkerForTest(
		store,
		sender,
		time.Now().UTC().Add(time.Hour),
	)
	if !worker.processOne(context.Background()) {
		t.Fatal("failed webhook item was not attempted")
	}
	row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
	if !found || row.lastError != discordErrorServer ||
		strings.Contains(row.lastError, secretBody) {
		t.Fatalf("row=%#v found=%t", row, found)
	}
}

func TestRecoveryDiscordWorkerProcessesMultipleRowsOneAtATime(t *testing.T) {
	store := openTestStore(t)
	first := insertRecoveryDiscordFixture(t, store, "multiple-a", true)
	second := insertRecoveryDiscordFixture(t, store, "multiple-bb", true)
	sender := &fakeRecoveryDiscordSender{}
	worker := newRecoveryDiscordWorkerForTest(
		store,
		sender,
		time.Now().UTC().Add(time.Hour),
	)
	if !worker.processOne(context.Background()) ||
		!worker.processOne(context.Background()) {
		t.Fatal("two pending rows were not both processed")
	}
	if worker.processOne(context.Background()) {
		t.Fatal("worker reported a third pending row")
	}
	if sender.callCount() != 2 {
		t.Fatalf("send calls = %d, want 2", sender.callCount())
	}
	for _, eventID := range []string{first.EventID, second.EventID} {
		row, found := readRecoveryDiscordOutboxRow(t, store, eventID)
		if !found || row.state != recoveryDiscordStateSent {
			t.Fatalf("event %s row=%#v found=%t", eventID, row, found)
		}
	}
}

func TestRecoveryDiscordWorkerMissingConfigurationDoesNotHotLoop(t *testing.T) {
	store := openTestStore(t)
	incident := insertRecoveryDiscordFixture(t, store, "missing-config", true)
	now := time.Now().UTC().Add(time.Hour)
	worker := newRecoveryDiscordWorkerForTest(store, nil, now)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()
	deadline := time.Now().Add(time.Second)
	for {
		row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
		if found && row.attemptCount == 1 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("missing-config attempt was not recorded")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop cleanly")
	}
	row, found := readRecoveryDiscordOutboxRow(t, store, incident.EventID)
	if !found || row.state != recoveryDiscordStateRetry ||
		row.attemptCount != 1 ||
		row.lastError != discordErrorInvalidConfiguration {
		t.Fatalf("missing-config row=%#v found=%t", row, found)
	}
}

func TestRecoveryDiscordWorkerCleanShutdownWhileIdle(t *testing.T) {
	store := openTestStore(t)
	worker := newRecoveryDiscordWorkerForTest(
		store,
		&fakeRecoveryDiscordSender{},
		time.Now().UTC(),
	)
	worker.pollInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle worker did not stop")
	}
}

func TestRecoveryDiscordRetryScheduleAndErrorBound(t *testing.T) {
	want := []time.Duration{
		time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		30 * time.Minute,
		time.Hour,
		time.Hour,
	}
	for index, delay := range want {
		if got := recoveryDiscordRetryDelay(index + 1); got != delay {
			t.Fatalf("attempt %d delay=%s, want %s", index+1, got, delay)
		}
	}
	raw := strings.Repeat("sensitive\n", 100)
	sanitized := sanitizeRecoveryDiscordLastError(raw)
	if len(sanitized) > maxRecoveryDiscordLastError ||
		strings.ContainsAny(sanitized, "\r\n") {
		t.Fatalf("sanitized error length=%d value=%q", len(sanitized), sanitized)
	}
}

func TestFormatRecoveryDiscordMessage(t *testing.T) {
	_, incident := recoveryDiscordFixture("format")
	protection := system.RecoveryProtectionState{
		State: system.RecoveryProtectionArmed,
		HardwareWatchdog: system.HardwareWatchdogProtectionState{
			State: system.RecoveryProtectionArmed,
		},
		RTC: system.RTCRecoveryProtectionState{
			State: system.RecoveryProtectionNotArmed,
		},
	}
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	message, err := FormatRecoveryDiscordMessage(incident, protection, location)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"**" + RecoveryDiscordTitle + "**",
		"Last known alive: " + incident.LastKnownAliveAt.In(location).Format("Jan 2, 2006 3:04 PM MST"),
		"Recovered: " + incident.RecoveredAt.In(location).Format("Jan 2, 2006 3:04 PM MST"),
		"Downtime: 3h 2m",
		"Status: Recovered",
		"Automatic recovery: Armed",
		"Hardware watchdog: Armed",
		"RTC recovery: Not armed",
		"ReactorLab is back online.",
	} {
		if !strings.Contains(message.Content, required) {
			t.Errorf("content missing %q:\n%s", required, message.Content)
		}
	}
	for _, forbidden := range []string{
		"Power lost at",
		"power lost",
		"Recovered by",
		"caused recovery",
		incident.PreviousBootID,
		incident.RecoveryBootID,
		"bootStatus",
		"discord-webhook",
		"/etc/",
		"{\"",
	} {
		if strings.Contains(message.Content, forbidden) {
			t.Errorf("content contains forbidden value %q:\n%s", forbidden, message.Content)
		}
	}
	if len(message.Content) > maxDiscordContentBytes {
		t.Fatalf("message length=%d", len(message.Content))
	}
}

func TestFormatRecoveryDiscordMessageExactExample(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	lastKnownAlive := time.Date(2026, 9, 21, 7, 32, 0, 0, time.UTC)
	recoveredAt := time.Date(2026, 9, 21, 15, 36, 0, 0, time.UTC)
	message, err := FormatRecoveryDiscordMessage(
		RecoveryIncident{
			EventID:          "example",
			LastKnownAliveAt: lastKnownAlive,
			RecoveredAt:      recoveredAt,
			DowntimeSeconds:  int64(recoveredAt.Sub(lastKnownAlive) / time.Second),
			Status:           recoveryIncidentStatus,
		},
		system.RecoveryProtectionState{
			State: system.RecoveryProtectionArmed,
			HardwareWatchdog: system.HardwareWatchdogProtectionState{
				State: system.RecoveryProtectionArmed,
			},
			RTC: system.RTCRecoveryProtectionState{
				State: system.RecoveryProtectionArmed,
			},
		},
		location,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "**ReactorLab recovered from an unexpected shutdown**\n\n" +
		"Last known alive: Sep 21, 2026 3:32 AM EDT\n" +
		"Recovered: Sep 21, 2026 11:36 AM EDT\n" +
		"Downtime: 8h 4m\n" +
		"Status: Recovered\n\n" +
		"Automatic recovery: Armed\n" +
		"Hardware watchdog: Armed\n" +
		"RTC recovery: Armed\n\n" +
		"ReactorLab is back online."
	if message.Content != want {
		t.Fatalf("content:\n%s\nwant:\n%s", message.Content, want)
	}
}

func TestFormatRecoveryDiscordMessageTimezoneIsDSTAware(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if got := formatRecoveryTimestamp(
		time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		location,
	); !strings.HasSuffix(got, " EDT") {
		t.Fatalf("summer timestamp=%q", got)
	}
	if got := formatRecoveryTimestamp(
		time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		location,
	); !strings.HasSuffix(got, " EST") {
		t.Fatalf("winter timestamp=%q", got)
	}
}

func TestFormatRecoveryDiscordMessageMissingProtectionDetails(t *testing.T) {
	_, incident := recoveryDiscordFixture("unavailable")
	message, err := FormatRecoveryDiscordMessage(
		incident,
		system.RecoveryProtectionState{},
		time.UTC,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"Automatic recovery: Unavailable",
		"Hardware watchdog: Unavailable",
		"RTC recovery: Unavailable",
	} {
		if !strings.Contains(message.Content, line) {
			t.Fatalf("content missing %q:\n%s", line, message.Content)
		}
	}
}
