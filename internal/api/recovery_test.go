package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/observability"
	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

func systemHandlerForRecoveryTest(t *testing.T, source *fakeObservability) *Handler {
	t.Helper()
	handler := newHandlerWithObservability(t.TempDir(), source).(*Handler)
	handler.collectSystem = func() (reactorsystem.Metrics, error) {
		return reactorsystem.Metrics{
			CPU:           reactorsystem.CPUStats{UsagePercent: 41.5},
			UptimeSeconds: 7200,
			CollectedAt:   time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
		}, nil
	}
	return handler
}

func TestAdminSystemIncludesRecoveryWithoutChangingExistingMetrics(t *testing.T) {
	lastKnownAlive := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	recoveredAt := time.Date(2026, 9, 21, 11, 0, 0, 0, time.UTC)
	wakeAt := time.Date(2026, 9, 21, 12, 5, 0, 0, time.UTC)
	bootStatus := uint32(0)
	source := &fakeObservability{latest: &observability.RecoveryIncident{
		EventID:          "event-1",
		LastKnownAliveAt: lastKnownAlive,
		RecoveredAt:      recoveredAt,
		DowntimeSeconds:  10800,
		Status:           "recovered",
		PreviousBootID:   "must-not-be-exposed",
		RecoveryBootID:   "must-not-be-exposed",
	}}
	handler := systemHandlerForRecoveryTest(t, source)
	handler.inspectHardwareWatchdogProtection = func(context.Context) (reactorsystem.HardwareWatchdogProtectionState, error) {
		return reactorsystem.HardwareWatchdogProtectionState{
			State:          reactorsystem.RecoveryProtectionArmed,
			Identity:       "SP5100 TCO timer",
			TimeoutSeconds: 60,
			BootStatus:     &bootStatus,
		}, nil
	}
	handler.inspectRTCRecoveryProtection = func(context.Context) (reactorsystem.RTCRecoveryProtectionState, error) {
		return reactorsystem.RTCRecoveryProtectionState{
			State: reactorsystem.RecoveryProtectionArmed, WakeAt: &wakeAt,
		}, nil
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/system", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body adminSystemResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CPU.UsagePercent != 41.5 || body.UptimeSeconds != 7200 {
		t.Fatalf("existing metrics changed: %#v", body.Metrics)
	}
	protection := body.Recovery.Protection
	if protection.State != reactorsystem.RecoveryProtectionArmed ||
		protection.HardwareWatchdog.State != reactorsystem.RecoveryProtectionArmed ||
		protection.HardwareWatchdog.Identity != "SP5100 TCO timer" ||
		protection.HardwareWatchdog.TimeoutSeconds != 60 ||
		protection.HardwareWatchdog.BootStatus == nil ||
		*protection.HardwareWatchdog.BootStatus != 0 ||
		protection.RTC.State != reactorsystem.RecoveryProtectionArmed ||
		protection.RTC.WakeAt == nil ||
		!protection.RTC.WakeAt.Equal(wakeAt) ||
		!body.Recovery.HistoryAvailable ||
		body.Recovery.LastIncident == nil {
		t.Fatalf("recovery = %#v", body.Recovery)
	}
	incident := body.Recovery.LastIncident
	if incident.EventID != "event-1" ||
		!incident.LastKnownAliveAt.Equal(lastKnownAlive) ||
		!incident.RecoveredAt.Equal(recoveredAt) ||
		incident.DowntimeSeconds != 10800 || incident.Status != "recovered" {
		t.Fatalf("incident = %#v", incident)
	}
	var raw map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	recovery := raw["recovery"].(map[string]any)
	if len(recovery) != 3 {
		t.Fatalf("recovery shape = %#v", recovery)
	}
	rawProtection := recovery["protection"].(map[string]any)
	if len(rawProtection) != 3 {
		t.Fatalf("protection shape = %#v", rawProtection)
	}
	rawHardware := rawProtection["hardwareWatchdog"].(map[string]any)
	if rawHardware["state"] != "armed" ||
		rawHardware["identity"] != "SP5100 TCO timer" ||
		rawHardware["timeoutSeconds"] != float64(60) ||
		rawHardware["bootStatus"] != float64(0) {
		t.Fatalf("hardware watchdog shape = %#v", rawHardware)
	}
	rawRTC := rawProtection["rtc"].(map[string]any)
	if rawRTC["state"] != "armed" ||
		rawRTC["wakeAt"] != wakeAt.Format(time.RFC3339) {
		t.Fatalf("RTC shape = %#v", rawRTC)
	}
	if _, exposed := rawProtection["wakeAt"]; exposed {
		t.Fatal("wakeAt was not moved under recovery.protection.rtc")
	}
	rawIncident := recovery["lastIncident"].(map[string]any)
	if _, exposed := rawIncident["previousBootId"]; exposed {
		t.Fatal("previous boot ID was exposed by /api/v1/system")
	}
	if _, exposed := rawIncident["recoveryBootId"]; exposed {
		t.Fatal("recovery boot ID was exposed by /api/v1/system")
	}
}

func TestAdminSystemRecoveryInspectionFailuresAreNonFatal(t *testing.T) {
	hardwareArmed := func(context.Context) (reactorsystem.HardwareWatchdogProtectionState, error) {
		return reactorsystem.HardwareWatchdogProtectionState{
			State: reactorsystem.RecoveryProtectionArmed,
		}, nil
	}
	rtcArmed := func(context.Context) (reactorsystem.RTCRecoveryProtectionState, error) {
		return reactorsystem.RTCRecoveryProtectionState{
			State: reactorsystem.RecoveryProtectionArmed,
		}, nil
	}
	hardwareFailed := func(context.Context) (reactorsystem.HardwareWatchdogProtectionState, error) {
		return reactorsystem.HardwareWatchdogProtectionState{}, errors.New("watchdog unavailable")
	}
	rtcFailed := func(context.Context) (reactorsystem.RTCRecoveryProtectionState, error) {
		return reactorsystem.RTCRecoveryProtectionState{}, errors.New("RTC unavailable")
	}

	tests := []struct {
		name            string
		inspectHardware func(context.Context) (reactorsystem.HardwareWatchdogProtectionState, error)
		inspectRTC      func(context.Context) (reactorsystem.RTCRecoveryProtectionState, error)
		wantAggregate   string
		wantHardware    string
		wantRTC         string
	}{
		{
			name:            "RTC failure",
			inspectHardware: hardwareArmed,
			inspectRTC:      rtcFailed,
			wantAggregate:   reactorsystem.RecoveryProtectionArmed,
			wantHardware:    reactorsystem.RecoveryProtectionArmed,
			wantRTC:         reactorsystem.RecoveryProtectionUnavailable,
		},
		{
			name:            "hardware failure",
			inspectHardware: hardwareFailed,
			inspectRTC:      rtcArmed,
			wantAggregate:   reactorsystem.RecoveryProtectionArmed,
			wantHardware:    reactorsystem.RecoveryProtectionUnavailable,
			wantRTC:         reactorsystem.RecoveryProtectionArmed,
		},
		{
			name:            "both failures",
			inspectHardware: hardwareFailed,
			inspectRTC:      rtcFailed,
			wantAggregate:   reactorsystem.RecoveryProtectionUnavailable,
			wantHardware:    reactorsystem.RecoveryProtectionUnavailable,
			wantRTC:         reactorsystem.RecoveryProtectionUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := systemHandlerForRecoveryTest(t, &fakeObservability{
				latestErr: errors.New("history unavailable"),
			})
			handler.inspectHardwareWatchdogProtection = test.inspectHardware
			handler.inspectRTCRecoveryProtection = test.inspectRTC

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/system", nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var body adminSystemResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			protection := body.Recovery.Protection
			if body.CPU.UsagePercent != 41.5 ||
				protection.State != test.wantAggregate ||
				protection.HardwareWatchdog.State != test.wantHardware ||
				protection.RTC.State != test.wantRTC ||
				body.Recovery.HistoryAvailable ||
				body.Recovery.LastIncident != nil {
				t.Fatalf("body = %#v", body)
			}
		})
	}
}
