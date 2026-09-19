package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/system"
)

const (
	hostBootStateKey        = "host.boot_id"
	serviceStatePrefix      = "service.health."
	serviceInvocationPrefix = "service.invocation."
)

type HostTracker struct {
	previous *system.HistoricalSnapshot
	bootID   string
}

func (t *HostTracker) SetBootBaseline(bootID string) { t.bootID = bootID }

func (t *HostTracker) Observe(current system.HistoricalSnapshot) (HostSample, []Event, map[string]string) {
	sample := HostSample{
		CollectedAt: current.ObservedAt, MemoryUsedBytes: current.MemoryUsedBytes,
		MemoryTotalBytes: current.MemoryTotalBytes, Load1: current.Load1, Load5: current.Load5,
		Load15: current.Load15, DiskUsedBytes: current.DiskUsedBytes,
		DiskTotalBytes: current.DiskTotalBytes, UptimeSeconds: current.UptimeSeconds,
		BootID: current.BootID,
	}
	if t.previous != nil {
		previous := *t.previous
		if current.CPUTotal > previous.CPUTotal && current.CPUIdle >= previous.CPUIdle {
			total := current.CPUTotal - previous.CPUTotal
			idle := current.CPUIdle - previous.CPUIdle
			if idle <= total {
				value := float64(total-idle) / float64(total) * 100
				if value < 0 {
					value = 0
				} else if value > 100 {
					value = 100
				}
				sample.CPUPercent = &value
			}
		}
		elapsed := current.ObservedAt.Sub(previous.ObservedAt)
		if current.NetworkInterface != "" && current.NetworkInterface == previous.NetworkInterface {
			sample.NetworkRXBPS = rate(previous.NetworkRXBytes, current.NetworkRXBytes, elapsed)
			sample.NetworkTXBPS = rate(previous.NetworkTXBytes, current.NetworkTXBytes, elapsed)
		}
		if previous.DiskReadBytes != nil && current.DiskReadBytes != nil {
			sample.DiskReadBPS = rate(*previous.DiskReadBytes, *current.DiskReadBytes, elapsed)
		}
		if previous.DiskWriteBytes != nil && current.DiskWriteBytes != nil {
			sample.DiskWriteBPS = rate(*previous.DiskWriteBytes, *current.DiskWriteBytes, elapsed)
		}
	}
	events := make([]Event, 0, 1)
	state := make(map[string]string)
	if t.bootID != "" && current.BootID != "" && t.bootID != current.BootID {
		sourceID := "boot:" + current.BootID
		events = append(events, newEvent("reactorlab", sourceID, "host_restart", "host", "dell", "Dell host", current.ObservedAt, "Dell host restarted", nil))
	}
	if current.BootID != "" && current.BootID != t.bootID {
		state[hostBootStateKey] = current.BootID
		t.bootID = current.BootID
	}
	copy := current
	t.previous = &copy
	return sample, events, state
}

type appCounter struct {
	at        time.Time
	signature string
	rx        uint64
	tx        uint64
	restarts  int
}

type ApplicationTracker struct {
	previous map[string]appCounter
}

func (t *ApplicationTracker) Observe(snapshot minideploy.Snapshot, observedAt time.Time) ([]ApplicationSample, []Event) {
	if t.previous == nil {
		t.previous = make(map[string]appCounter)
	}
	if !snapshot.CollectedAt.IsZero() {
		observedAt = snapshot.CollectedAt.UTC()
	} else {
		observedAt = observedAt.UTC()
	}
	next := make(map[string]appCounter, len(snapshot.Deployments))
	samples := make([]ApplicationSample, 0, len(snapshot.Deployments))
	events := make([]Event, 0)
	for _, deployment := range snapshot.Deployments {
		containers := append([]minideploy.ContainerMetrics(nil), deployment.Containers...)
		sort.Slice(containers, func(i, j int) bool { return containers[i].Container < containers[j].Container })
		var cpu float64
		var memoryUsed, memoryLimit, rx, tx uint64
		var restarts int
		names := make([]string, 0, len(containers))
		for _, container := range containers {
			cpu += container.CPUPercent
			memoryUsed += container.MemoryUsedBytes
			memoryLimit += container.MemoryLimitBytes
			rx += container.NetworkRXBytes
			tx += container.NetworkTXBytes
			restarts += container.RestartCount
			names = append(names, container.Container)
		}
		signature := strings.Join(names, "\x00")
		current := appCounter{at: observedAt, signature: signature, rx: rx, tx: tx, restarts: restarts}
		sample := ApplicationSample{
			CollectedAt: observedAt, AppID: deployment.App, Name: deployment.App,
			Status: deployment.Status, CPUPercent: cpu, MemoryUsedBytes: memoryUsed,
			MemoryLimitBytes: memoryLimit, RestartCount: restarts,
		}
		if previous, ok := t.previous[deployment.App]; ok && previous.signature == signature {
			elapsed := observedAt.Sub(previous.at)
			sample.NetworkRXBPS = rate(previous.rx, rx, elapsed)
			sample.NetworkTXBPS = rate(previous.tx, tx, elapsed)
			if restarts > previous.restarts {
				sourceID := fmt.Sprintf("app:%s:%s:restarts:%d", deployment.App, signature, restarts)
				events = append(events, newEvent("minideploy_runtime", sourceID,
					"application_restart", "application", deployment.App, deployment.App,
					observedAt, fmt.Sprintf("%s restart count increased to %d", deployment.App, restarts),
					map[string]any{"restartCount": restarts}))
			}
		}
		next[deployment.App] = current
		samples = append(samples, sample)
	}
	t.previous = next
	return samples, events
}

type ServiceTracker struct {
	health     map[string]bool
	healthSeen map[string]bool
	invocation map[string]string
}

func (t *ServiceTracker) SetHealthBaseline(id string, available bool) {
	t.ensure()
	t.health[id], t.healthSeen[id] = available, true
}

func (t *ServiceTracker) SetInvocationBaseline(id, invocation string) {
	t.ensure()
	if invocation != "" {
		t.invocation[id] = invocation
	}
}

func (t *ServiceTracker) Observe(samples []ServiceSample) ([]Event, map[string]string) {
	t.ensure()
	events := make([]Event, 0)
	state := make(map[string]string)
	for _, sample := range samples {
		previousHealth, healthSeen := t.health[sample.ServiceID], t.healthSeen[sample.ServiceID]
		if healthSeen && previousHealth != sample.Available {
			eventType, summary := "service_outage", sample.Name+" became unavailable"
			if sample.Available {
				eventType, summary = "service_recovery", sample.Name+" recovered"
			}
			sourceID := fmt.Sprintf("health:%s:%t:%d", sample.ServiceID, sample.Available, sample.CollectedAt.UTC().UnixMilli())
			events = append(events, newEvent("reactorlab", sourceID, eventType, "service",
				sample.ServiceID, sample.Name, sample.CollectedAt, summary, nil))
		}
		t.health[sample.ServiceID], t.healthSeen[sample.ServiceID] = sample.Available, true
		if !healthSeen || previousHealth != sample.Available {
			state[serviceStatePrefix+sample.ServiceID] = fmt.Sprintf("%t", sample.Available)
		}
		if sample.RestartIdentity != "" {
			previous := t.invocation[sample.ServiceID]
			if previous != "" && previous != sample.RestartIdentity {
				sourceID := "restart:" + sample.ServiceID + ":" + sample.RestartIdentity
				events = append(events, newEvent("systemd", sourceID, "platform_service_restart",
					"service", sample.ServiceID, sample.Name, sample.CollectedAt,
					sample.Name+" restarted", nil))
			}
			t.invocation[sample.ServiceID] = sample.RestartIdentity
			if previous != sample.RestartIdentity {
				state[serviceInvocationPrefix+sample.ServiceID] = sample.RestartIdentity
			}
		}
	}
	return events, state
}

func (t *ServiceTracker) ensure() {
	if t.health == nil {
		t.health = make(map[string]bool)
		t.healthSeen = make(map[string]bool)
		t.invocation = make(map[string]string)
	}
}

func MiniBaseEvents(items []minibase.Activity) []Event {
	events := make([]Event, 0, len(items))
	for _, item := range items {
		eventType := ""
		switch item.Type {
		case "database_create":
			eventType = "database_creation"
		case "database_delete":
			eventType = "database_deletion"
		case "backup_create", "automatic_backup":
			eventType = "database_backup"
		case "backup_restore_new", "backup_restore_replace":
			eventType = "database_restore"
		default:
			continue
		}
		resourceID, resourceName := item.DatabaseID, item.DatabaseDisplayName
		if resourceName == "" {
			resourceName = "MiniBase"
		}
		immutable := strings.Join([]string{item.Type, item.Outcome, item.Source, item.DatabaseID,
			item.DatabaseDisplayName, item.Detail, item.CreatedAt.UTC().Format(time.RFC3339Nano)}, "\x00")
		sourceID := digest(immutable)
		events = append(events, newEvent("minibase_activity", sourceID, eventType, "database",
			resourceID, resourceName, item.CreatedAt, item.Detail,
			map[string]any{"outcome": item.Outcome, "operationSource": item.Source}))
	}
	return events
}

func newEvent(source, sourceID, eventType, resourceType, resourceID, resourceName string, at time.Time, summary string, details map[string]any) Event {
	return Event{
		ID: digest(source + "\x00" + sourceID), Source: source, SourceEventID: sourceID,
		Type: eventType, ResourceType: resourceType, ResourceID: resourceID,
		ResourceName: resourceName, OccurredAt: at.UTC(), Summary: summary, Details: details,
	}
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
