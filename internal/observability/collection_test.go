package observability

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/system"
)

func TestTemperatureAccumulatorPreservesSpikeAndPartialBucket(t *testing.T) {
	start := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	var accumulator TemperatureAccumulator
	for index, value := range []float64{58, 61, 87, 91, 64} {
		accumulator.Add(start.Add(time.Duration(index)*time.Second), value)
	}
	bucket, ok := accumulator.Flush(start.Add(5 * time.Second))
	if !ok || bucket.SampleCount != 5 || bucket.MinCelsius != 58 || bucket.MaxCelsius != 91 || bucket.AvgCelsius != 72.2 || !bucket.PeakAt.Equal(start.Add(3*time.Second)) {
		t.Fatalf("bucket = %#v, ok = %v", bucket, ok)
	}
	accumulator.Add(start, 70)
	partial, ok := accumulator.Flush(start.Add(2 * time.Second))
	if !ok || partial.SampleCount != 1 {
		t.Fatalf("partial = %#v, ok = %v", partial, ok)
	}
	if _, ok := accumulator.Flush(start); ok {
		t.Fatal("empty accumulator produced a bucket")
	}
}

func TestRateFirstSampleAndResetDoNotFabricate(t *testing.T) {
	if got := rate(100, 200, 10*time.Second); got == nil || *got != 10 {
		t.Fatalf("rate = %v", got)
	}
	if got := rate(200, 100, time.Second); got != nil {
		t.Fatalf("reset rate = %v", *got)
	}
	tracker := HostTracker{}
	now := time.Now().UTC()
	first, _, _ := tracker.Observe(system.HistoricalSnapshot{ObservedAt: now, CPUTotal: 100, CPUIdle: 50, NetworkInterface: "eth0", NetworkRXBytes: 100, NetworkTXBytes: 100, BootID: "boot-a"})
	if first.CPUPercent != nil || first.NetworkRXBPS != nil {
		t.Fatalf("first sample fabricated rates: %#v", first)
	}
	second, _, _ := tracker.Observe(system.HistoricalSnapshot{ObservedAt: now.Add(5 * time.Second), CPUTotal: 200, CPUIdle: 80, NetworkInterface: "eth0", NetworkRXBytes: 200, NetworkTXBytes: 150, BootID: "boot-a"})
	if second.CPUPercent == nil || *second.CPUPercent != 70 || second.NetworkRXBPS == nil || *second.NetworkRXBPS != 20 {
		t.Fatalf("second sample = %#v", second)
	}
}

func TestHostBootBaselineDoesNotFabricateAndChangeEmitsOnce(t *testing.T) {
	now := time.Now().UTC()
	tracker := HostTracker{}
	_, events, _ := tracker.Observe(system.HistoricalSnapshot{ObservedAt: now, BootID: "boot-a"})
	if len(events) != 0 {
		t.Fatal("first boot baseline emitted event")
	}
	_, events, _ = tracker.Observe(system.HistoricalSnapshot{ObservedAt: now.Add(time.Second), BootID: "boot-b"})
	if len(events) != 1 || events[0].Type != "host_restart" {
		t.Fatalf("events = %#v", events)
	}
	_, events, _ = tracker.Observe(system.HistoricalSnapshot{ObservedAt: now.Add(2 * time.Second), BootID: "boot-b"})
	if len(events) != 0 {
		t.Fatal("unchanged boot ID emitted duplicate")
	}
}

func TestApplicationMetricsStaySeparateAndReplacementResetsRates(t *testing.T) {
	now := time.Now().UTC()
	tracker := ApplicationTracker{}
	first := minideploy.Snapshot{CollectedAt: now, Deployments: []minideploy.Deployment{
		{App: "alpha", Status: "healthy", Containers: []minideploy.ContainerMetrics{{Container: "alpha-1", CPUPercent: 2, MemoryUsedBytes: 3, NetworkRXBytes: 100, NetworkTXBytes: 200, RestartCount: 1}}},
		{App: "beta", Status: "stopped", Containers: []minideploy.ContainerMetrics{{Container: "beta-1", CPUPercent: 4, MemoryUsedBytes: 5, NetworkRXBytes: 20, NetworkTXBytes: 30}}},
	}}
	samples, events := tracker.Observe(first, now)
	if len(samples) != 2 || samples[0].AppID == samples[1].AppID || len(events) != 0 || samples[0].NetworkRXBPS != nil {
		t.Fatalf("first observation = %#v, events = %#v", samples, events)
	}
	second := first
	second.CollectedAt = now.Add(10 * time.Second)
	second.Deployments[0].Containers[0].NetworkRXBytes = 200
	second.Deployments[0].Containers[0].NetworkTXBytes = 250
	second.Deployments[0].Containers[0].RestartCount = 2
	samples, events = tracker.Observe(second, second.CollectedAt)
	if samples[0].NetworkRXBPS == nil || *samples[0].NetworkRXBPS != 10 || len(events) != 1 || events[0].Type != "application_restart" {
		t.Fatalf("second observation = %#v, events = %#v", samples, events)
	}
	second.CollectedAt = now.Add(20 * time.Second)
	second.Deployments[0].Containers[0].Container = "alpha-2"
	samples, _ = tracker.Observe(second, second.CollectedAt)
	if samples[0].NetworkRXBPS != nil || samples[0].NetworkTXBPS != nil {
		t.Fatalf("replacement retained network baseline: %#v", samples[0])
	}
}

func TestServiceTransitionsAndRestartsAreDeduplicated(t *testing.T) {
	now := time.Now().UTC()
	tracker := ServiceTracker{}
	healthy := []ServiceSample{{CollectedAt: now, ServiceID: "minibase", Name: "MiniBase", Available: true, Status: "healthy", RestartIdentity: "a"}}
	events, _ := tracker.Observe(healthy)
	if len(events) != 0 {
		t.Fatal("initial state emitted event")
	}
	unhealthy := []ServiceSample{{CollectedAt: now.Add(time.Second), ServiceID: "minibase", Name: "MiniBase", Available: false, Status: "unavailable", RestartIdentity: "a"}}
	events, _ = tracker.Observe(unhealthy)
	if len(events) != 1 || events[0].Type != "service_outage" {
		t.Fatalf("outage events = %#v", events)
	}
	events, _ = tracker.Observe(unhealthy)
	if len(events) != 0 {
		t.Fatal("repeated outage emitted duplicate")
	}
	recovered := []ServiceSample{{CollectedAt: now.Add(2 * time.Second), ServiceID: "minibase", Name: "MiniBase", Available: true, Status: "healthy", RestartIdentity: "b"}}
	events, _ = tracker.Observe(recovered)
	if len(events) != 2 || events[0].Type != "service_recovery" || events[1].Type != "platform_service_restart" {
		t.Fatalf("recovery events = %#v", events)
	}
}

func TestMiniBaseEventsUseDeterministicAuthoritativeIdentity(t *testing.T) {
	at := time.Now().UTC()
	items := []minibase.Activity{{DatabaseID: "db-a", DatabaseDisplayName: "Primary", Type: "backup_create", Outcome: "success", Source: "admin", Detail: "Backup created", CreatedAt: at}}
	first := MiniBaseEvents(items)
	second := MiniBaseEvents(items)
	if len(first) != 1 || first[0].Type != "database_backup" || first[0].ID != second[0].ID || first[0].SourceEventID != second[0].SourceEventID {
		t.Fatalf("events are not stable: %#v / %#v", first, second)
	}
	if got := MiniBaseEvents([]minibase.Activity{{Type: "attachment_attach", CreatedAt: at}}); len(got) != 0 {
		t.Fatalf("unsupported activity created event: %#v", got)
	}
}

func TestRunPeriodicSerializesAndStops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	var active, maximum, calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		runPeriodic(ctx, 2*time.Millisecond, true, func() {
			current := active.Add(1)
			for {
				old := maximum.Load()
				if current <= old || maximum.CompareAndSwap(old, current) {
					break
				}
			}
			calls.Add(1)
			time.Sleep(5 * time.Millisecond)
			active.Add(-1)
		})
	}()
	<-done
	if maximum.Load() != 1 || calls.Load() < 2 {
		t.Fatalf("maximum = %d, calls = %d", maximum.Load(), calls.Load())
	}
}
