package system

import "testing"

func TestAggregateRecoveryProtectionState(t *testing.T) {
	tests := []struct {
		name             string
		hardwareWatchdog string
		rtc              string
		want             string
	}{
		{"hardware armed and RTC armed", RecoveryProtectionArmed, RecoveryProtectionArmed, RecoveryProtectionArmed},
		{"hardware armed and RTC not armed", RecoveryProtectionArmed, RecoveryProtectionNotArmed, RecoveryProtectionArmed},
		{"hardware armed and RTC unavailable", RecoveryProtectionArmed, RecoveryProtectionUnavailable, RecoveryProtectionArmed},
		{"hardware not armed and RTC armed", RecoveryProtectionNotArmed, RecoveryProtectionArmed, RecoveryProtectionArmed},
		{"hardware unavailable and RTC armed", RecoveryProtectionUnavailable, RecoveryProtectionArmed, RecoveryProtectionArmed},
		{"hardware not armed and RTC not armed", RecoveryProtectionNotArmed, RecoveryProtectionNotArmed, RecoveryProtectionNotArmed},
		{"hardware unavailable and RTC not armed", RecoveryProtectionUnavailable, RecoveryProtectionNotArmed, RecoveryProtectionNotArmed},
		{"hardware not armed and RTC unavailable", RecoveryProtectionNotArmed, RecoveryProtectionUnavailable, RecoveryProtectionNotArmed},
		{"hardware unavailable and RTC unavailable", RecoveryProtectionUnavailable, RecoveryProtectionUnavailable, RecoveryProtectionUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := AggregateRecoveryProtectionState(
				test.hardwareWatchdog,
				test.rtc,
			)
			if got != test.want {
				t.Fatalf("aggregate state = %q, want %q", got, test.want)
			}
		})
	}
}
