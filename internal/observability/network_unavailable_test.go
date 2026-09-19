package observability

import (
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/system"
)

func TestUnavailableHostInterfaceNeverBecomesFabricatedZeroRate(t *testing.T) {
	now := time.Now().UTC()
	tracker := HostTracker{}
	tracker.Observe(system.HistoricalSnapshot{ObservedAt: now, CPUTotal: 100, CPUIdle: 50})
	sample, _, _ := tracker.Observe(system.HistoricalSnapshot{ObservedAt: now.Add(5 * time.Second), CPUTotal: 200, CPUIdle: 100})
	if sample.NetworkRXBPS != nil || sample.NetworkTXBPS != nil {
		t.Fatalf("unavailable interface produced rates: %#v", sample)
	}
}
