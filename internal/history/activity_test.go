package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestActivityQueries(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	first := ActivityEvent{
		OccurredAt:  time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC),
		Source:      "system",
		Kind:        "warning",
		Severity:    "warning",
		Subject:     "Memory usage",
		Message:     "Memory usage is high.",
		Fingerprint: "system:memory",
	}
	second := ActivityEvent{
		OccurredAt:  first.OccurredAt.Add(time.Minute),
		Source:      "system",
		Kind:        "recovery",
		Severity:    "info",
		Subject:     "Memory usage",
		Message:     "Memory usage recovered.",
		Fingerprint: "system:memory",
	}

	if err := store.AppendActivity(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendActivity(ctx, second); err != nil {
		t.Fatal(err)
	}

	latest, found, err := store.LatestActivity(ctx, "system:memory")
	if err != nil {
		t.Fatal(err)
	}
	if !found || latest.Kind != "recovery" {
		t.Fatalf("unexpected latest activity %#v found=%v", latest, found)
	}

	events, err := store.ListActivity(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Kind != "recovery" || events[1].Kind != "warning" {
		t.Fatalf("events not newest-first: %#v", events)
	}
	if events[0].ID == 0 {
		t.Fatal("expected persisted activity id")
	}

	events, err = store.ListActivity(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected limit to apply, got %d", len(events))
	}
}

func TestListActivityRejectsInvalidLimitByNormalizing(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	events, err := store.ListActivity(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if events == nil {
		t.Fatal("expected empty slice, got nil")
	}

	events, err = store.ListActivity(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if events == nil {
		t.Fatal("expected empty slice, got nil")
	}
}
