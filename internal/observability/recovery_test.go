package observability

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func encodedHostSession(t *testing.T, session HostSession) string {
	t.Helper()
	value, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	return string(value)
}

func TestHostSessionFirstStartupEstablishesBaseline(t *testing.T) {
	observedAt := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	var tracker HostSessionTracker
	update, err := tracker.Observe("boot-a", observedAt, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if update.Event != nil || update.Incident != nil || !update.Critical {
		t.Fatalf("unexpected first update: %#v", update)
	}
	session, err := decodeHostSession(update.StateValue)
	if err != nil {
		t.Fatal(err)
	}
	if session.BootID != "boot-a" ||
		!session.BootedAt.Equal(observedAt.Add(-time.Hour)) ||
		!session.LastSeenAt.Equal(observedAt) || session.CleanShutdown {
		t.Fatalf("session = %#v", session)
	}
}

func TestHostSessionSameBootRestartReturnsSessionToDirty(t *testing.T) {
	bootedAt := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	previous := HostSession{
		BootID: "boot-a", BootedAt: bootedAt,
		LastSeenAt: bootedAt.Add(time.Hour), CleanShutdown: true,
	}
	var tracker HostSessionTracker
	if err := tracker.SetBaseline(encodedHostSession(t, previous)); err != nil {
		t.Fatal(err)
	}
	observedAt := bootedAt.Add(2 * time.Hour)
	update, err := tracker.Observe("boot-a", observedAt, 7200)
	if err != nil {
		t.Fatal(err)
	}
	if update.Event != nil || !update.Critical {
		t.Fatalf("same-boot update = %#v", update)
	}
	session, err := decodeHostSession(update.StateValue)
	if err != nil {
		t.Fatal(err)
	}
	if session.CleanShutdown || !session.BootedAt.Equal(bootedAt) ||
		!session.LastSeenAt.Equal(observedAt) {
		t.Fatalf("session = %#v", session)
	}
}

func TestHostSessionCleanPreviousShutdownCreatesNoRecovery(t *testing.T) {
	previous := HostSession{
		BootID:        "boot-a",
		BootedAt:      time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
		LastSeenAt:    time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC),
		CleanShutdown: true,
	}
	var tracker HostSessionTracker
	if err := tracker.SetBaseline(encodedHostSession(t, previous)); err != nil {
		t.Fatal(err)
	}
	update, err := tracker.Observe(
		"boot-b",
		time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
		3600,
	)
	if err != nil {
		t.Fatal(err)
	}
	if update.Event != nil || update.Incident != nil || !update.Critical {
		t.Fatalf("clean reboot update = %#v", update)
	}
}

func TestHostSessionDirtyPreviousShutdownCreatesRecovery(t *testing.T) {
	lastKnownAlive := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	previous := HostSession{
		BootID: "boot-a", BootedAt: lastKnownAlive.Add(-time.Hour),
		LastSeenAt: lastKnownAlive,
	}
	var tracker HostSessionTracker
	if err := tracker.SetBaseline(encodedHostSession(t, previous)); err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	update, err := tracker.Observe("boot-b", observedAt, 3600)
	if err != nil {
		t.Fatal(err)
	}
	recoveredAt := observedAt.Add(-time.Hour)
	if update.Event == nil || update.Incident == nil {
		t.Fatalf("dirty reboot update = %#v", update)
	}
	if update.Event.Type != recoveryEventType ||
		update.Event.Source != "reactorlab" ||
		update.Event.ResourceType != "host" ||
		update.Event.ResourceID != "dell" ||
		!update.Event.OccurredAt.Equal(recoveredAt) {
		t.Fatalf("event = %#v", update.Event)
	}
	incident := update.Incident
	if !incident.LastKnownAliveAt.Equal(lastKnownAlive) ||
		!incident.RecoveredAt.Equal(recoveredAt) ||
		incident.DowntimeSeconds != int64(3*time.Hour/time.Second) ||
		incident.Status != recoveryIncidentStatus ||
		incident.PreviousBootID != "boot-a" ||
		incident.RecoveryBootID != "boot-b" {
		t.Fatalf("incident = %#v", incident)
	}
}

func TestHostSessionRecoveryIsDeterministicAndPersistedOnce(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	previous := HostSession{
		BootID:     "boot-a",
		BootedAt:   time.Date(2026, 9, 21, 6, 0, 0, 0, time.UTC),
		LastSeenAt: time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
	}
	observe := func() HostSessionUpdate {
		var tracker HostSessionTracker
		if err := tracker.SetBaseline(encodedHostSession(t, previous)); err != nil {
			t.Fatal(err)
		}
		update, err := tracker.Observe(
			"boot-b",
			time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
			3600,
		)
		if err != nil {
			t.Fatal(err)
		}
		return update
	}
	first, second := observe(), observe()
	if first.Event == nil || second.Event == nil || first.Event.ID != second.Event.ID {
		t.Fatalf("event identities differ: %#v / %#v", first.Event, second.Event)
	}
	for _, update := range []HostSessionUpdate{first, second} {
		if err := store.InsertBatch(ctx, Batch{
			Events: []Event{*update.Event},
			State:  map[string]string{hostSessionStateKey: update.StateValue},
		}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := store.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM events WHERE event_type = ?",
		recoveryEventType,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("recovery event count = %d, want 1", count)
	}

	persisted, ok, err := store.State(ctx, hostSessionStateKey)
	if err != nil || !ok {
		t.Fatalf("state found = %v, err = %v", ok, err)
	}
	var restarted HostSessionTracker
	if err := restarted.SetBaseline(persisted); err != nil {
		t.Fatal(err)
	}
	repeat, err := restarted.Observe(
		"boot-b",
		time.Date(2026, 9, 21, 12, 0, 5, 0, time.UTC),
		3605,
	)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.Event != nil {
		t.Fatalf("same recovered boot emitted another event: %#v", repeat.Event)
	}
}

func TestHostSessionClockAnomalyTransitionsWithoutIncident(t *testing.T) {
	previous := HostSession{
		BootID:     "boot-a",
		BootedAt:   time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC),
		LastSeenAt: time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC),
	}
	var tracker HostSessionTracker
	if err := tracker.SetBaseline(encodedHostSession(t, previous)); err != nil {
		t.Fatal(err)
	}
	update, err := tracker.Observe(
		"boot-b",
		time.Date(2026, 9, 21, 10, 30, 0, 0, time.UTC),
		3600,
	)
	if !errors.Is(err, ErrInvalidRecoveryClock) {
		t.Fatalf("err = %v, want ErrInvalidRecoveryClock", err)
	}
	if update.Event != nil || update.StateValue == "" || !update.Critical {
		t.Fatalf("clock anomaly update = %#v", update)
	}
	session, decodeErr := decodeHostSession(update.StateValue)
	if decodeErr != nil || session.BootID != "boot-b" {
		t.Fatalf("session = %#v, err = %v", session, decodeErr)
	}
}

func TestHostSessionRejectsMalformedOrMissingBootData(t *testing.T) {
	var tracker HostSessionTracker
	if err := tracker.SetBaseline("{"); err == nil {
		t.Fatal("malformed session was accepted")
	}
	invalid := encodedHostSession(t, HostSession{
		BootedAt:   time.Now().Add(-time.Hour),
		LastSeenAt: time.Now(),
	})
	if err := tracker.SetBaseline(invalid); !errors.Is(err, ErrInvalidPreviousSession) {
		t.Fatalf("missing prior boot id err = %v", err)
	}
	if _, err := tracker.Observe("", time.Now(), 10); !errors.Is(err, ErrInvalidHostObservation) {
		t.Fatalf("missing current boot id err = %v", err)
	}
	if _, err := tracker.Observe("boot-a", time.Now(), -1); !errors.Is(err, ErrInvalidHostObservation) {
		t.Fatalf("negative uptime err = %v", err)
	}
}

func TestHostSessionMarkClean(t *testing.T) {
	var tracker HostSessionTracker
	if _, ok, err := tracker.MarkClean(); err != nil || ok {
		t.Fatalf("empty tracker clean result ok=%v err=%v", ok, err)
	}
	previous := HostSession{
		BootID:        "boot-a",
		BootedAt:      time.Now().UTC().Add(-time.Hour),
		LastSeenAt:    time.Now().UTC(),
		CleanShutdown: false,
	}
	if err := tracker.SetBaseline(encodedHostSession(t, previous)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := tracker.MarkClean(); err != nil || ok {
		t.Fatalf("unobserved baseline clean result ok=%v err=%v", ok, err)
	}
	if _, err := tracker.Observe("boot-a", time.Now().UTC(), 60); err != nil {
		t.Fatal(err)
	}
	value, ok, err := tracker.MarkClean()
	if err != nil || !ok {
		t.Fatalf("mark clean ok=%v err=%v", ok, err)
	}
	session, err := decodeHostSession(value)
	if err != nil {
		t.Fatal(err)
	}
	if !session.CleanShutdown {
		t.Fatalf("session = %#v, want clean", session)
	}
}

func TestRecoveryEventAndSessionStateAreAtomic(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.InsertBatch(ctx, Batch{
		State: map[string]string{hostSessionStateKey: "previous"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertBatch(ctx, Batch{
		Events: []Event{{}},
		State:  map[string]string{hostSessionStateKey: "must-not-commit"},
	}); err == nil {
		t.Fatal("invalid event unexpectedly committed")
	}
	value, ok, err := store.State(ctx, hostSessionStateKey)
	if err != nil || !ok || value != "previous" {
		t.Fatalf("state = %q found=%v err=%v", value, ok, err)
	}
}

func TestLatestRecoveryIncident(t *testing.T) {
	store := openTestStore(t)
	previous := HostSession{
		BootID:     "boot-a",
		BootedAt:   time.Date(2026, 9, 21, 6, 0, 0, 0, time.UTC),
		LastSeenAt: time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
	}
	var tracker HostSessionTracker
	if err := tracker.SetBaseline(encodedHostSession(t, previous)); err != nil {
		t.Fatal(err)
	}
	update, err := tracker.Observe(
		"boot-b",
		time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
		3600,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertBatch(context.Background(), Batch{
		Events: []Event{*update.Event},
		State:  map[string]string{hostSessionStateKey: update.StateValue},
	}); err != nil {
		t.Fatal(err)
	}
	query := NewQueryService(store)
	incident, err := query.LatestRecoveryIncident(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if incident == nil || update.Incident == nil ||
		incident.EventID != update.Incident.EventID ||
		!incident.LastKnownAliveAt.Equal(update.Incident.LastKnownAliveAt) ||
		!incident.RecoveredAt.Equal(update.Incident.RecoveredAt) ||
		incident.DowntimeSeconds != update.Incident.DowntimeSeconds {
		t.Fatalf("incident = %#v, want %#v", incident, update.Incident)
	}

	incidents, err := query.ListRecoveryIncidents(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 || incidents[0].EventID != incident.EventID {
		t.Fatalf("incidents = %#v", incidents)
	}
	byID, err := query.RecoveryIncidentByID(context.Background(), incident.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if byID == nil || byID.EventID != incident.EventID {
		t.Fatalf("incident by ID = %#v", byID)
	}
	missing, err := query.RecoveryIncidentByID(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("missing incident = %#v", missing)
	}
}
