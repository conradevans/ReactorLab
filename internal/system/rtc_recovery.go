package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	RecoveryProtectionArmed       = "armed"
	RecoveryProtectionNotArmed    = "not_armed"
	RecoveryProtectionUnavailable = "unavailable"

	rtcRecoveryUnit      = "rtc-recovery-watchdog.service"
	rtcWakeAlarmPath     = "/sys/class/rtc/rtc0/wakealarm"
	rtcMinimumWakeLead   = 3 * time.Minute
	rtcMaximumWakeLead   = 7 * time.Minute
	rtcInspectionTimeout = 2 * time.Second
)

type RTCRecoveryProtectionState struct {
	State  string     `json:"state"`
	WakeAt *time.Time `json:"wakeAt,omitempty"`
}

type rtcUnitInspector func(context.Context) (map[string]string, error)
type rtcWakeAlarmReader func(string) ([]byte, error)

func InspectRTCRecoveryProtection(ctx context.Context) (RTCRecoveryProtectionState, error) {
	inspectionContext, cancel := context.WithTimeout(ctx, rtcInspectionTimeout)
	defer cancel()
	return inspectRTCRecoveryProtection(
		inspectionContext,
		time.Now().UTC(),
		inspectRTCRecoveryUnit,
		os.ReadFile,
	)
}

func inspectRTCRecoveryProtection(
	ctx context.Context,
	now time.Time,
	inspectUnit rtcUnitInspector,
	readWakeAlarm rtcWakeAlarmReader,
) (RTCRecoveryProtectionState, error) {
	unavailable := RTCRecoveryProtectionState{State: RecoveryProtectionUnavailable}
	properties, err := inspectUnit(ctx)
	if err != nil {
		return unavailable, fmt.Errorf("inspect RTC recovery service: %w", err)
	}
	loadState, loadOK := properties["LoadState"]
	activeState, activeOK := properties["ActiveState"]
	subState, subOK := properties["SubState"]
	unitFileState, unitFileOK := properties["UnitFileState"]
	if !loadOK || !activeOK || !subOK || !unitFileOK {
		return unavailable, errors.New("inspect RTC recovery service: incomplete systemd properties")
	}
	if loadState != "loaded" {
		return RTCRecoveryProtectionState{State: RecoveryProtectionNotArmed}, nil
	}
	enabled := unitFileState == "enabled" || unitFileState == "enabled-runtime"
	if !enabled || activeState != "active" || subState != "running" {
		return RTCRecoveryProtectionState{State: RecoveryProtectionNotArmed}, nil
	}

	data, err := readWakeAlarm(rtcWakeAlarmPath)
	if err != nil {
		return unavailable, fmt.Errorf("read RTC wake alarm: %w", err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" || value == "0" {
		return RTCRecoveryProtectionState{State: RecoveryProtectionNotArmed}, nil
	}
	epoch, err := strconv.ParseInt(value, 10, 64)
	if err != nil || epoch <= 0 {
		if err == nil {
			err = errors.New("wake alarm must be a positive Unix timestamp")
		}
		return unavailable, fmt.Errorf("parse RTC wake alarm: %w", err)
	}
	wakeAt := time.Unix(epoch, 0).UTC()
	state := RTCRecoveryProtectionState{State: RecoveryProtectionNotArmed, WakeAt: &wakeAt}
	lead := wakeAt.Sub(now.UTC())
	if lead < rtcMinimumWakeLead || lead > rtcMaximumWakeLead {
		return state, nil
	}
	state.State = RecoveryProtectionArmed
	return state, nil
}

func inspectRTCRecoveryUnit(ctx context.Context) (map[string]string, error) {
	output, err := exec.CommandContext(
		ctx,
		"systemctl",
		"show",
		"--no-pager",
		"--property=LoadState",
		"--property=ActiveState",
		"--property=SubState",
		"--property=UnitFileState",
		rtcRecoveryUnit,
	).Output()
	if err != nil {
		return nil, err
	}
	properties := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			properties[key] = strings.TrimSpace(value)
		}
	}
	return properties, nil
}
