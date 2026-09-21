package system

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
)

func healthyRTCUnitProperties() map[string]string {
	return map[string]string{
		"LoadState":     "loaded",
		"ActiveState":   "active",
		"SubState":      "running",
		"UnitFileState": "enabled",
	}
}

func staticRTCInspector(properties map[string]string, err error) rtcUnitInspector {
	return func(context.Context) (map[string]string, error) {
		return properties, err
	}
}

func staticWakeAlarm(value string, err error) rtcWakeAlarmReader {
	return func(string) ([]byte, error) {
		return []byte(value), err
	}
}

func TestInspectRTCRecoveryProtectionArmed(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	wakeAt := now.Add(5 * time.Minute)
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		now,
		staticRTCInspector(healthyRTCUnitProperties(), nil),
		staticWakeAlarm(strconv.FormatInt(wakeAt.Unix(), 10), nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	if state.State != RecoveryProtectionArmed || state.WakeAt == nil ||
		!state.WakeAt.Equal(wakeAt) {
		t.Fatalf("state = %#v, want armed at %s", state, wakeAt)
	}
}

func TestInspectRTCRecoveryProtectionServiceInactive(t *testing.T) {
	properties := healthyRTCUnitProperties()
	properties["ActiveState"] = "inactive"
	properties["SubState"] = "dead"
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		time.Now(),
		staticRTCInspector(properties, nil),
		staticWakeAlarm("", errors.New("must not be read")),
	)
	if err != nil || state.State != RecoveryProtectionNotArmed {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectRTCRecoveryProtectionServiceDisabled(t *testing.T) {
	properties := healthyRTCUnitProperties()
	properties["UnitFileState"] = "disabled"
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		time.Now(),
		staticRTCInspector(properties, nil),
		staticWakeAlarm("", errors.New("must not be read")),
	)
	if err != nil || state.State != RecoveryProtectionNotArmed {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectRTCRecoveryProtectionMissingWakeAlarm(t *testing.T) {
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		time.Now(),
		staticRTCInspector(healthyRTCUnitProperties(), nil),
		staticWakeAlarm("", os.ErrNotExist),
	)
	if err == nil || state.State != RecoveryProtectionUnavailable {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectRTCRecoveryProtectionStaleWakeAlarm(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	wakeAt := now.Add(-time.Minute)
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		now,
		staticRTCInspector(healthyRTCUnitProperties(), nil),
		staticWakeAlarm(strconv.FormatInt(wakeAt.Unix(), 10), nil),
	)
	if err != nil || state.State != RecoveryProtectionNotArmed ||
		state.WakeAt == nil || !state.WakeAt.Equal(wakeAt) {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectRTCRecoveryProtectionOutsideRollingWindow(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	wakeAt := now.Add(10 * time.Minute)
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		now,
		staticRTCInspector(healthyRTCUnitProperties(), nil),
		staticWakeAlarm(strconv.FormatInt(wakeAt.Unix(), 10), nil),
	)
	if err != nil || state.State != RecoveryProtectionNotArmed {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectRTCRecoveryProtectionMalformedWakeAlarm(t *testing.T) {
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		time.Now(),
		staticRTCInspector(healthyRTCUnitProperties(), nil),
		staticWakeAlarm("not-a-timestamp", nil),
	)
	if err == nil || state.State != RecoveryProtectionUnavailable {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectRTCRecoveryProtectionUnavailable(t *testing.T) {
	state, err := inspectRTCRecoveryProtection(
		context.Background(),
		time.Now(),
		staticRTCInspector(nil, errors.New("systemd unavailable")),
		staticWakeAlarm("", nil),
	)
	if err == nil || state.State != RecoveryProtectionUnavailable {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}
