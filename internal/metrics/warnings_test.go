package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/history"
)

func TestReconcileWarningDeduplicatesAndRecovers(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	sampler := &Sampler{
		store: store,
		now:   func() time.Time { return now },
	}

	condition := warningCondition{
		Active:          true,
		Source:          "system",
		Subject:         "Memory usage",
		Message:         "Memory usage is 95.0%.",
		RecoveryMessage: "Memory usage returned below the warning threshold.",
		Fingerprint:     "system:memory",
	}

	if err := sampler.reconcileWarning(context.Background(), condition); err != nil {
		t.Fatal(err)
	}
	if err := sampler.reconcileWarning(context.Background(), condition); err != nil {
		t.Fatal(err)
	}
	if len(store.activity) != 1 {
		t.Fatalf("expected one deduplicated warning, got %d", len(store.activity))
	}
	if store.activity[0].Kind != "warning" {
		t.Fatalf("expected warning event, got %#v", store.activity[0])
	}

	condition.Active = false
	if err := sampler.reconcileWarning(context.Background(), condition); err != nil {
		t.Fatal(err)
	}
	if len(store.activity) != 2 {
		t.Fatalf("expected recovery event, got %d events", len(store.activity))
	}
	if store.activity[1].Kind != "recovery" || store.activity[1].Severity != "info" {
		t.Fatalf("unexpected recovery event %#v", store.activity[1])
	}
}

func TestWarningConditionsCoverThresholdsAndHealth(t *testing.T) {
	batch := history.Batch{
		System: history.SystemSample{
			MemoryUsagePercent: 95,
			DiskUsagePercent:   90,
			TemperatureCelsius: 88,
		},
		Deployments: []history.DeploymentSample{
			{
				App:           "broken-app",
				Status:        "degraded",
				DatabaseState: "conflict",
			},
		},
		Databases: []history.DatabaseSample{
			{
				ID:              "database_1",
				DisplayName:     "Broken Database",
				Status:          "error",
				DeploymentState: "unresolved",
			},
		},
	}

	conditions := warningConditions(batch, true, true)
	active := make(map[string]bool)
	for _, condition := range conditions {
		active[condition.Fingerprint] = condition.Active
	}

	for _, fingerprint := range []string{
		"system:memory",
		"system:disk",
		"system:temperature",
		"deployment:broken-app:status",
		"deployment:broken-app:database-relationship",
		"database:database_1:status",
		"database:database_1:deployment-relationship",
	} {
		if !active[fingerprint] {
			t.Fatalf("expected active warning %q", fingerprint)
		}
	}
}

func TestWarningConditionsDoNotResolveRelationshipsDuringSourceOutage(t *testing.T) {
	batch := history.Batch{
		Deployments: []history.DeploymentSample{
			{App: "app", Status: "healthy", DatabaseState: "unavailable"},
		},
		Databases: []history.DatabaseSample{
			{ID: "db", DisplayName: "DB", Status: "ready", DeploymentState: "unavailable"},
		},
	}

	conditions := warningConditions(batch, false, false)
	for _, condition := range conditions {
		if condition.Fingerprint == "deployment:app:database-relationship" ||
			condition.Fingerprint == "database:db:deployment-relationship" {
			t.Fatalf("relationship condition should be skipped during source outage: %#v", condition)
		}
	}
}
