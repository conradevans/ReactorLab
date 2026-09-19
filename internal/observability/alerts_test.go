package observability

import (
	"context"
	"testing"
	"time"
)

func TestAlertRepositoryRoundTripSupportsDurationAndIncidentState(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	rule := AlertRule{
		ID: "cpu-sustained", Scope: "host", Metric: "cpu_percent", Operator: ">",
		Threshold: 90, DurationSeconds: 300, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.PutAlertRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	incident := AlertIncident{
		ID: "incident-a", RuleID: rule.ID, ResourceType: "host", ResourceID: "dell",
		OpenedAt: now, LastObservedAt: now.Add(5 * time.Minute), State: "open",
	}
	if err := store.PutAlertIncident(ctx, incident); err != nil {
		t.Fatal(err)
	}
	rules, err := store.ListAlertRules(ctx)
	if err != nil || len(rules) != 1 || rules[0].DurationSeconds != 300 || !rules[0].Enabled {
		t.Fatalf("rules = %#v, err = %v", rules, err)
	}
	incidents, err := store.ListAlertIncidents(ctx, "open")
	if err != nil || len(incidents) != 1 || incidents[0].LastObservedAt.Sub(incidents[0].OpenedAt) != 5*time.Minute {
		t.Fatalf("incidents = %#v, err = %v", incidents, err)
	}
}
