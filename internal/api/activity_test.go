package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/history"
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
