package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/observability"
)

type fakeActivitySource struct {
	events []history.ActivityEvent
	err    error
}

func (f fakeActivitySource) ListActivity(
	context.Context,
	int,
) ([]history.ActivityEvent, error) {
	return f.events, f.err
}

func TestAdminActivityReturnsSafeEvents(t *testing.T) {
	handler := newHandlerWithAllSources(
		"",
		nil,
		nil,
		fakeActivitySource{
			events: []history.ActivityEvent{
				{
					ID:          7,
					OccurredAt:  time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC),
					Source:      "minideploy",
					Kind:        "warning",
					Severity:    "warning",
					Subject:     "example",
					Message:     "Deployment status is degraded.",
					Fingerprint: "internal:fingerprint",
				},
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/activity", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "fingerprint") ||
		strings.Contains(response.Body.String(), "internal:fingerprint") {
		t.Fatalf("activity response leaked internal fingerprint: %s", response.Body.String())
	}

	var payload activityResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 1 {
		t.Fatalf("expected one event, got %d", len(payload.Events))
	}
	if payload.Events[0].Subject != "example" ||
		payload.Events[0].Kind != "warning" {
		t.Fatalf("unexpected event %#v", payload.Events[0])
	}
}

func TestAdminActivityUnavailable(t *testing.T) {
	handler := newHandlerWithAllSources(
		"",
		nil,
		nil,
		fakeActivitySource{err: errors.New("down")},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/activity", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)

	}
}
func activityHandlerWithRecovery(
	activity activitySource,
	source observabilitySource,
) http.Handler {
	handler := newHandlerWithAllSources("", nil, nil, activity).(*Handler)
	handler.observability = source
	return handler
}

func recoveryIncidentFixture(eventID string, recoveredAt time.Time) observability.RecoveryIncident {
	return observability.RecoveryIncident{
		EventID:          eventID,
		LastKnownAliveAt: recoveredAt.Add(-4*time.Minute - 24*time.Second),
		RecoveredAt:      recoveredAt,
		DowntimeSeconds:  264,
		Status:           "recovered",
		PreviousBootID:   "private-previous-boot",
		RecoveryBootID:   "private-recovery-boot",
	}
}

func TestAdminActivityMergesDurableRecoveryNewestFirst(t *testing.T) {
	legacyAt := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	recoveredAt := legacyAt.Add(time.Hour)
	eventID := strings.Repeat("a", 64)
	source := &fakeObservability{recoveries: []observability.RecoveryIncident{
		recoveryIncidentFixture(eventID, recoveredAt),
	}}
	handler := activityHandlerWithRecovery(
		fakeActivitySource{events: []history.ActivityEvent{{
			ID: 9, OccurredAt: legacyAt, Source: "system", Kind: "warning",
			Severity: "warning", Subject: "Legacy event", Message: "Still available.",
		}}},
		source,
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/activity", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload activityResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 2 || payload.Events[0].EventID != eventID ||
		payload.Events[1].ID != 9 {
		t.Fatalf("merged events = %#v", payload.Events)
	}
	recovery := payload.Events[0]
	if recovery.Kind != "unexpected_shutdown_recovery" ||
		recovery.Subject != "Unexpected shutdown detected" || recovery.Incident == nil ||
		recovery.Incident.DowntimeSeconds != 264 || recovery.Incident.Status != "recovered" {
		t.Fatalf("recovery event = %#v", recovery)
	}
	for _, forbidden := range []string{
		"previousBootId", "recoveryBootId", "private-previous-boot",
		"private-recovery-boot", "sourceEventId",
	} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("Activity response exposed %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestAdminActivityIncludesRequestedOldRecoveryIncident(t *testing.T) {
	base := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	recent := make([]observability.RecoveryIncident, activityResultLimit)
	for index := range recent {
		recent[index] = recoveryIncidentFixture(
			fmt.Sprintf("%064x", index+1),
			base.Add(-time.Duration(index)*time.Minute),
		)
	}
	requestedID := strings.Repeat("f", 64)
	requested := recoveryIncidentFixture(requestedID, base.Add(-30*24*time.Hour))
	source := &fakeObservability{recoveries: recent, recoveryByID: &requested}
	handler := activityHandlerWithRecovery(fakeActivitySource{}, source)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/activity?event="+requestedID,
		nil,
	))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload activityResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != activityResultLimit {
		t.Fatalf("event count = %d, want %d", len(payload.Events), activityResultLimit)
	}
	if !activityContainsEventID(payload.Events, requestedID) {
		t.Fatalf("requested event %q was not retained", requestedID)
	}
	if source.lookupID != requestedID {
		t.Fatalf("lookup ID = %q", source.lookupID)
	}
}

func TestAdminActivityRejectsMalformedRequestedEvent(t *testing.T) {
	handler := activityHandlerWithRecovery(fakeActivitySource{}, &fakeObservability{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/activity?event=../../private",
		nil,
	))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAdminActivityNonexistentRequestedEventDegradesNormally(t *testing.T) {
	requestedID := strings.Repeat("e", 64)
	source := &fakeObservability{}
	handler := activityHandlerWithRecovery(fakeActivitySource{events: []history.ActivityEvent{{
		ID: 4, OccurredAt: time.Now().UTC(), Source: "system", Kind: "warning",
		Severity: "warning", Subject: "Legacy event", Message: "Still available.",
	}}}, source)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/activity?event="+requestedID,
		nil,
	))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload activityResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 1 || payload.Events[0].ID != 4 {
		t.Fatalf("events = %#v", payload.Events)
	}
}

func TestAdminActivityDurableFailurePreservesLegacyHistory(t *testing.T) {
	handler := activityHandlerWithRecovery(
		fakeActivitySource{events: []history.ActivityEvent{{
			ID: 5, OccurredAt: time.Now().UTC(), Source: "system", Kind: "warning",
			Severity: "warning", Subject: "Legacy event", Message: "Still available.",
		}}},
		&fakeObservability{recoveryErr: errors.New("observability unavailable")},
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/activity", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload activityResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 1 || payload.Events[0].ID != 5 {
		t.Fatalf("events = %#v", payload.Events)
	}
}
