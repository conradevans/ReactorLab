package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/observability"
)

type fakeObservability struct {
	lastRange    observability.Range
	latest       *observability.RecoveryIncident
	latestErr    error
	recoveries   []observability.RecoveryIncident
	recoveryErr  error
	recoveryByID *observability.RecoveryIncident
	lookupErr    error
	lookupID     string
}

func (f *fakeObservability) QueryHost(_ context.Context, window observability.Range) ([]observability.HostPoint, error) {
	f.lastRange = window
	value := 50.0
	return []observability.HostPoint{{Timestamp: window.From, SampleCount: 1, CPUAverage: &value, CPUMaximum: &value}}, nil
}

func (f *fakeObservability) QueryTemperature(_ context.Context, window observability.Range) ([]observability.TemperaturePoint, error) {
	f.lastRange = window
	return []observability.TemperaturePoint{}, nil
}

func (f *fakeObservability) QueryApplications(_ context.Context, window observability.Range) ([]observability.ApplicationSummary, error) {
	f.lastRange = window
	return []observability.ApplicationSummary{}, nil
}

func (f *fakeObservability) QueryApplication(_ context.Context, _ string, window observability.Range) ([]observability.ApplicationPoint, error) {
	f.lastRange = window
	return []observability.ApplicationPoint{}, nil
}

func (f *fakeObservability) QueryServices(_ context.Context, window observability.Range) ([]observability.ServiceSeries, error) {
	f.lastRange = window
	return []observability.ServiceSeries{}, nil
}

func (f *fakeObservability) QueryEvents(_ context.Context, from, to time.Time, _ int) ([]observability.Event, error) {
	f.lastRange = observability.Range{From: from, To: to}
	return []observability.Event{}, nil
}
func (f *fakeObservability) LatestRecoveryIncident(context.Context) (*observability.RecoveryIncident, error) {
	return f.latest, f.latestErr
}

func (f *fakeObservability) ListRecoveryIncidents(context.Context, int) ([]observability.RecoveryIncident, error) {
	return f.recoveries, f.recoveryErr
}

func (f *fakeObservability) RecoveryIncidentByID(_ context.Context, eventID string) (*observability.RecoveryIncident, error) {
	f.lookupID = eventID
	return f.recoveryByID, f.lookupErr
}

func TestObservabilityHostRouteIsBoundedAndSafe(t *testing.T) {
	source := &fakeObservability{}
	handler := newHandlerWithObservability(t.TempDir(), source)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/observability/host?range=7d", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if source.lastRange.Name != "7d" || source.lastRange.MaxPoints > observability.MaxDisplayPoints {
		t.Fatalf("range = %#v", source.lastRange)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["data"]; !ok {
		t.Fatalf("response lacks typed data envelope: %#v", body)
	}
	for _, forbidden := range []string{"DATABASE_URL", "environment", "token", "credential"} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("unsafe key %q returned", forbidden)
		}
	}
}

func TestObservabilityRejectsUnknownRange(t *testing.T) {
	handler := newHandlerWithObservability(t.TempDir(), &fakeObservability{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/observability/host?range=30d", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestObservabilityStorageFailureIsUnavailable(t *testing.T) {
	handler := newHandlerWithObservability(t.TempDir(), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/observability/host?range=1h", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestPublicGuestHandlerDoesNotExposeObservability(t *testing.T) {
	handler := newPublicHandler(t.TempDir(), nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/observability/host?range=1h", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}
