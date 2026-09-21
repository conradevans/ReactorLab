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
	hardwareWatchdogSysfsRoot         = "/sys/class/watchdog/watchdog0"
	hardwareWatchdogInspectionTimeout = 2 * time.Second
)

type hardwareWatchdogFileReader func(string) ([]byte, error)
type runtimeWatchdogInspector func(context.Context) (string, error)

func InspectHardwareWatchdogProtection(
	ctx context.Context,
) (HardwareWatchdogProtectionState, error) {
	inspectionContext, cancel := context.WithTimeout(
		ctx,
		hardwareWatchdogInspectionTimeout,
	)
	defer cancel()
	return inspectHardwareWatchdogProtection(
		inspectionContext,
		os.ReadFile,
		inspectSystemdRuntimeWatchdog,
	)
}

func inspectHardwareWatchdogProtection(
	ctx context.Context,
	readFile hardwareWatchdogFileReader,
	inspectRuntimeWatchdog runtimeWatchdogInspector,
) (HardwareWatchdogProtectionState, error) {
	unavailable := HardwareWatchdogProtectionState{
		State: RecoveryProtectionUnavailable,
	}
	identityData, err := readFile(hardwareWatchdogSysfsRoot + "/identity")
	if errors.Is(err, os.ErrNotExist) {
		return HardwareWatchdogProtectionState{
			State: RecoveryProtectionNotArmed,
		}, nil
	}
	if err != nil {
		return unavailable, fmt.Errorf("read hardware watchdog identity: %w", err)
	}
	identity := strings.TrimSpace(string(identityData))
	if identity == "" {
		return unavailable, errors.New("read hardware watchdog identity: empty value")
	}

	state := HardwareWatchdogProtectionState{
		State:    RecoveryProtectionNotArmed,
		Identity: identity,
	}
	stateData, err := readFile(hardwareWatchdogSysfsRoot + "/state")
	if err != nil {
		return unavailable, fmt.Errorf("read hardware watchdog state: %w", err)
	}
	switch strings.TrimSpace(string(stateData)) {
	case "inactive":
		return state, nil
	case "active":
	default:
		return unavailable, errors.New("read hardware watchdog state: unrecognized value")
	}

	timeoutData, err := readFile(hardwareWatchdogSysfsRoot + "/timeout")
	if err != nil {
		return unavailable, fmt.Errorf("read hardware watchdog timeout: %w", err)
	}
	timeout, err := strconv.ParseUint(strings.TrimSpace(string(timeoutData)), 10, 32)
	if err != nil || timeout == 0 {
		return state, nil
	}
	state.TimeoutSeconds = uint32(timeout)

	runtimeWatchdog, err := inspectRuntimeWatchdog(ctx)
	if err != nil {
		return unavailable, fmt.Errorf("inspect systemd runtime watchdog: %w", err)
	}
	enabled, err := runtimeWatchdogEnabled(runtimeWatchdog)
	if err != nil {
		return unavailable, fmt.Errorf("inspect systemd runtime watchdog: %w", err)
	}
	if !enabled {
		return state, nil
	}

	if bootStatusData, readErr := readFile(
		hardwareWatchdogSysfsRoot + "/bootstatus",
	); readErr == nil {
		bootStatus, parseErr := strconv.ParseUint(
			strings.TrimSpace(string(bootStatusData)),
			10,
			32,
		)
		if parseErr == nil {
			value := uint32(bootStatus)
			state.BootStatus = &value
		}
	}

	state.State = RecoveryProtectionArmed
	return state, nil
}

func inspectSystemdRuntimeWatchdog(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(
		ctx,
		"systemctl",
		"show",
		"--no-pager",
		"--property=RuntimeWatchdogUSec",
	).Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && key == "RuntimeWatchdogUSec" {
			return strings.TrimSpace(value), nil
		}
	}
	return "", errors.New("RuntimeWatchdogUSec property missing")
}

func runtimeWatchdogEnabled(value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, errors.New("empty RuntimeWatchdogUSec value")
	}
	if numeric, err := strconv.ParseUint(value, 10, 64); err == nil {
		return numeric > 0, nil
	}

	remaining := value
	enabled := false
	for remaining != "" {
		remaining = strings.TrimLeft(remaining, " \t")
		if remaining == "" {
			break
		}
		numberEnd := 0
		dotSeen := false
		for numberEnd < len(remaining) {
			character := remaining[numberEnd]
			if character >= '0' && character <= '9' {
				numberEnd++
				continue
			}
			if character == '.' && !dotSeen {
				dotSeen = true
				numberEnd++
				continue
			}
			break
		}
		if numberEnd == 0 {
			return false, fmt.Errorf("invalid RuntimeWatchdogUSec value %q", value)
		}
		number, err := strconv.ParseFloat(remaining[:numberEnd], 64)
		if err != nil {
			return false, fmt.Errorf("invalid RuntimeWatchdogUSec value %q", value)
		}
		remaining = remaining[numberEnd:]
		unitLength := systemdDurationUnitLength(remaining)
		if unitLength == 0 {
			return false, fmt.Errorf("invalid RuntimeWatchdogUSec value %q", value)
		}
		if number > 0 {
			enabled = true
		}
		remaining = remaining[unitLength:]
		if remaining != "" && remaining[0] != ' ' && remaining[0] != '\t' {
			return false, fmt.Errorf("invalid RuntimeWatchdogUSec value %q", value)
		}
	}
	return enabled, nil
}

func systemdDurationUnitLength(value string) int {
	for _, unit := range []string{"min", "usec", "msec", "us", "ms", "h", "s"} {
		if strings.HasPrefix(value, unit) {
			return len(unit)
		}
	}
	return 0
}
