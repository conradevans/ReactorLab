package system

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func watchdogValues() map[string]string {
	return map[string]string{
		hardwareWatchdogSysfsRoot + "/identity":   "SP5100 TCO timer\n",
		hardwareWatchdogSysfsRoot + "/state":      "active\n",
		hardwareWatchdogSysfsRoot + "/timeout":    "60\n",
		hardwareWatchdogSysfsRoot + "/bootstatus": "0\n",
	}
}

func copyWatchdogValues(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for path, value := range values {
		copied[path] = value
	}
	return copied
}

func watchdogReader(
	values map[string]string,
	failures map[string]error,
	paths *[]string,
) hardwareWatchdogFileReader {
	return func(path string) ([]byte, error) {
		if paths != nil {
			*paths = append(*paths, path)
		}
		if err := failures[path]; err != nil {
			return nil, err
		}
		value, ok := values[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(value), nil
	}
}

func staticRuntimeWatchdog(
	value string,
	err error,
) runtimeWatchdogInspector {
	return func(context.Context) (string, error) {
		return value, err
	}
}

func TestInspectHardwareWatchdogProtectionArmed(t *testing.T) {
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(watchdogValues(), nil, nil),
		staticRuntimeWatchdog("1min", nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	if state.State != RecoveryProtectionArmed ||
		state.Identity != "SP5100 TCO timer" ||
		state.TimeoutSeconds != 60 ||
		state.BootStatus == nil ||
		*state.BootStatus != 0 {
		t.Fatalf("state = %#v", state)
	}
}

func TestInspectHardwareWatchdogProtectionDeviceMissing(t *testing.T) {
	managerCalled := false
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(nil, nil, nil),
		func(context.Context) (string, error) {
			managerCalled = true
			return "1min", nil
		},
	)
	if err != nil || state.State != RecoveryProtectionNotArmed {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
	if managerCalled {
		t.Fatal("systemd manager was inspected for an absent watchdog")
	}
}

func TestInspectHardwareWatchdogProtectionInactive(t *testing.T) {
	values := watchdogValues()
	values[hardwareWatchdogSysfsRoot+"/state"] = "inactive\n"
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(values, nil, nil),
		staticRuntimeWatchdog("", errors.New("must not be called")),
	)
	if err != nil || state.State != RecoveryProtectionNotArmed {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectHardwareWatchdogProtectionInvalidTimeout(t *testing.T) {
	for _, timeout := range []string{"0", "not-a-number", "4294967296"} {
		t.Run(timeout, func(t *testing.T) {
			values := watchdogValues()
			values[hardwareWatchdogSysfsRoot+"/timeout"] = timeout
			state, err := inspectHardwareWatchdogProtection(
				context.Background(),
				watchdogReader(values, nil, nil),
				staticRuntimeWatchdog("", errors.New("must not be called")),
			)
			if err != nil || state.State != RecoveryProtectionNotArmed {
				t.Fatalf("state = %#v, err = %v", state, err)
			}
		})
	}
}

func TestInspectHardwareWatchdogProtectionSystemdDisabled(t *testing.T) {
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(watchdogValues(), nil, nil),
		staticRuntimeWatchdog("0", nil),
	)
	if err != nil || state.State != RecoveryProtectionNotArmed {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectHardwareWatchdogProtectionMalformedRequiredValues(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		value string
	}{
		{
			name:  "empty identity",
			path:  hardwareWatchdogSysfsRoot + "/identity",
			value: " \n",
		},
		{
			name:  "unknown state",
			path:  hardwareWatchdogSysfsRoot + "/state",
			value: "surprising\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := watchdogValues()
			values[test.path] = test.value
			state, err := inspectHardwareWatchdogProtection(
				context.Background(),
				watchdogReader(values, nil, nil),
				staticRuntimeWatchdog("1min", nil),
			)
			if err == nil || state.State != RecoveryProtectionUnavailable {
				t.Fatalf("state = %#v, err = %v", state, err)
			}
		})
	}
}

func TestInspectHardwareWatchdogProtectionSysfsReadFailure(t *testing.T) {
	values := watchdogValues()
	for _, path := range []string{
		hardwareWatchdogSysfsRoot + "/identity",
		hardwareWatchdogSysfsRoot + "/state",
		hardwareWatchdogSysfsRoot + "/timeout",
	} {
		t.Run(path, func(t *testing.T) {
			state, err := inspectHardwareWatchdogProtection(
				context.Background(),
				watchdogReader(values, map[string]error{
					path: os.ErrPermission,
				}, nil),
				staticRuntimeWatchdog("1min", nil),
			)
			if err == nil || state.State != RecoveryProtectionUnavailable {
				t.Fatalf("state = %#v, err = %v", state, err)
			}
		})
	}
}

func TestInspectHardwareWatchdogProtectionSystemctlFailure(t *testing.T) {
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(watchdogValues(), nil, nil),
		staticRuntimeWatchdog("", errors.New("systemctl failed")),
	)
	if err == nil || state.State != RecoveryProtectionUnavailable {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectHardwareWatchdogProtectionBootStatusUnavailableStillArmed(
	t *testing.T,
) {
	values := copyWatchdogValues(watchdogValues())
	delete(values, hardwareWatchdogSysfsRoot+"/bootstatus")
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(values, nil, nil),
		staticRuntimeWatchdog("1min", nil),
	)
	if err != nil || state.State != RecoveryProtectionArmed ||
		state.BootStatus != nil {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectHardwareWatchdogProtectionBootStatusIsBounded(t *testing.T) {
	values := watchdogValues()
	values[hardwareWatchdogSysfsRoot+"/bootstatus"] = "4294967296"
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(values, nil, nil),
		staticRuntimeWatchdog("1min", nil),
	)
	if err != nil || state.State != RecoveryProtectionArmed ||
		state.BootStatus != nil {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestInspectHardwareWatchdogProtectionNeverReadsDeviceNode(t *testing.T) {
	var paths []string
	state, err := inspectHardwareWatchdogProtection(
		context.Background(),
		watchdogReader(watchdogValues(), nil, &paths),
		staticRuntimeWatchdog("1min", nil),
	)
	if err != nil || state.State != RecoveryProtectionArmed {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
	if len(paths) == 0 {
		t.Fatal("no sysfs files were read")
	}
	for _, path := range paths {
		if strings.HasPrefix(path, "/dev/watchdog") {
			t.Fatalf("device node was read: %q", path)
		}
		if !strings.HasPrefix(path, hardwareWatchdogSysfsRoot+"/") {
			t.Fatalf("read escaped watchdog sysfs: %q", path)
		}
	}
}

func TestRuntimeWatchdogEnabled(t *testing.T) {
	tests := []struct {
		value     string
		want      bool
		wantError bool
	}{
		{value: "1min", want: true},
		{value: "1min 30s", want: true},
		{value: "60000000", want: true},
		{value: "0", want: false},
		{value: "0us", want: false},
		{value: "", wantError: true},
		{value: "disabled", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := runtimeWatchdogEnabled(test.value)
			if (err != nil) != test.wantError || got != test.want {
				t.Fatalf(
					"runtimeWatchdogEnabled(%q) = %v, %v; want %v, error %v",
					test.value,
					got,
					err,
					test.want,
					test.wantError,
				)
			}
		})
	}
}
