package system

type HardwareWatchdogProtectionState struct {
	State          string  `json:"state"`
	Identity       string  `json:"identity,omitempty"`
	TimeoutSeconds uint32  `json:"timeoutSeconds,omitempty"`
	BootStatus     *uint32 `json:"bootStatus,omitempty"`
}

type RecoveryProtectionState struct {
	State            string                          `json:"state"`
	HardwareWatchdog HardwareWatchdogProtectionState `json:"hardwareWatchdog"`
	RTC              RTCRecoveryProtectionState      `json:"rtc"`
}

func AggregateRecoveryProtectionState(hardwareWatchdog, rtc string) string {
	if hardwareWatchdog == RecoveryProtectionArmed || rtc == RecoveryProtectionArmed {
		return RecoveryProtectionArmed
	}
	if hardwareWatchdog == RecoveryProtectionNotArmed || rtc == RecoveryProtectionNotArmed {
		return RecoveryProtectionNotArmed
	}
	return RecoveryProtectionUnavailable
}

func ValidRecoveryProtectionState(state string) bool {
	return state == RecoveryProtectionArmed ||
		state == RecoveryProtectionNotArmed ||
		state == RecoveryProtectionUnavailable
}
