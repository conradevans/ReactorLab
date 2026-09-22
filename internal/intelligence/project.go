package intelligence

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/observability"
	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

func ProjectSystem(metrics reactorsystem.Metrics) SystemSnapshot {
	services := make([]ServiceState, 0, len(metrics.Services))
	for _, service := range metrics.Services {
		services = append(services, ServiceState{
			Name: service.Name, Status: service.Status, Active: service.Active,
		})
	}
	return SystemSnapshot{
		CPU: CPUState{
			UsagePercent: metrics.CPU.UsagePercent,
			LogicalCores: metrics.CPU.LogicalCores,
			Load1:        metrics.CPU.Load1,
			Load5:        metrics.CPU.Load5,
			Load15:       metrics.CPU.Load15,
		},
		Memory: MemoryState{
			TotalBytes:       metrics.Memory.TotalBytes,
			UsedBytes:        metrics.Memory.UsedBytes,
			AvailableBytes:   metrics.Memory.AvailableBytes,
			UsagePercent:     metrics.Memory.UsagePercent,
			SwapTotalBytes:   metrics.Memory.SwapTotalBytes,
			SwapUsedBytes:    metrics.Memory.SwapUsedBytes,
			SwapUsagePercent: metrics.Memory.SwapUsagePercent,
		},
		Disk: DiskState{
			TotalBytes:     metrics.Disk.TotalBytes,
			UsedBytes:      metrics.Disk.UsedBytes,
			AvailableBytes: metrics.Disk.AvailableBytes,
			UsagePercent:   metrics.Disk.UsagePercent,
		},
		Temperature: TemperatureState{
			Celsius: metrics.Temperature.Celsius,
			Source:  metrics.Temperature.Source,
		},
		UptimeSeconds: metrics.UptimeSeconds,
		Network: NetworkState{
			Interface:     metrics.Network.Interface,
			RXBytes:       metrics.Network.RXBytes,
			TXBytes:       metrics.Network.TXBytes,
			RXBytesPerSec: metrics.Network.RXBytesPerSec,
			TXBytesPerSec: metrics.Network.TXBytesPerSec,
		},
		Battery: BatteryState{
			Available:   metrics.Battery.Available,
			Percent:     metrics.Battery.Percent,
			ACAvailable: metrics.Battery.ACAvailable,
			ACConnected: metrics.Battery.ACConnected,
			Status:      metrics.Battery.Status,
		},
		Services:    services,
		CollectedAt: metrics.CollectedAt.UTC(),
	}
}

func ProjectProtection(
	state reactorsystem.RecoveryProtectionState,
) ProtectionState {
	var wakeAt *time.Time
	if state.RTC.WakeAt != nil {
		value := state.RTC.WakeAt.UTC()
		wakeAt = &value
	}
	var bootStatus *uint32
	if state.HardwareWatchdog.BootStatus != nil {
		value := *state.HardwareWatchdog.BootStatus
		bootStatus = &value
	}
	return ProtectionState{
		State: state.State,
		HardwareWatchdog: HardwareWatchdogState{
			State:          state.HardwareWatchdog.State,
			Identity:       state.HardwareWatchdog.Identity,
			TimeoutSeconds: state.HardwareWatchdog.TimeoutSeconds,
			BootStatus:     bootStatus,
		},
		RTC: RTCRecoveryState{
			State:  state.RTC.State,
			WakeAt: wakeAt,
		},
	}
}

func ProjectRecoveryIncident(
	incident observability.RecoveryIncident,
) RecoveryIncident {
	return RecoveryIncident{
		EventID:          incident.EventID,
		LastKnownAliveAt: incident.LastKnownAliveAt.UTC(),
		RecoveredAt:      incident.RecoveredAt.UTC(),
		DowntimeSeconds:  incident.DowntimeSeconds,
		Status:           incident.Status,
	}
}

func ProjectDeployments(
	deployments []minideploy.DeploymentMetadata,
) DeploymentCollection {
	items := make([]Deployment, 0, len(deployments))
	for _, deployment := range deployments {
		services := make(
			[]DeploymentService,
			0,
			len(deployment.Services),
		)
		for _, service := range deployment.Services {
			services = append(services, DeploymentService{
				Name:     service.Name,
				Strategy: service.Strategy,
				Status:   service.Status,
				ImageID:  service.ImageID,
			})
		}
		databases := make(
			[]DeploymentDatabase,
			0,
			len(deployment.DatabaseAttachments),
		)
		for _, database := range deployment.DatabaseAttachments {
			databases = append(databases, DeploymentDatabase{
				ID:          database.DatabaseID,
				DisplayName: database.DisplayName,
				BindingName: database.BindingName,
			})
		}
		items = append(items, Deployment{
			App:         deployment.App,
			Strategy:    deployment.Strategy,
			Status:      deployment.Status,
			Source:      projectSource(deployment.Source),
			ActivatedAt: cloneTime(deployment.ActivatedAt),
			ImageID:     deployment.ImageID,
			Services:    services,
			Databases:   databases,
		})
	}
	return DeploymentCollection{Deployments: items}
}

func ProjectDeploymentHistory(
	history minideploy.DeploymentHistory,
) DeploymentHistory {
	versions := make([]DeploymentVersion, 0, len(history.Versions))
	for _, version := range history.Versions {
		services := make(
			[]DeploymentService,
			0,
			len(version.Services),
		)
		for _, service := range version.Services {
			services = append(services, DeploymentService{
				Name:     service.Name,
				Strategy: service.Strategy,
				Status:   service.Status,
				ImageID:  service.ImageID,
			})
		}
		versions = append(versions, DeploymentVersion{
			App:         version.App,
			Strategy:    version.Strategy,
			Source:      projectSource(version.Source),
			ActivatedAt: cloneTime(version.ActivatedAt),
			ArchivedAt:  version.ArchivedAt.UTC(),
			ImageID:     version.ImageID,
			Services:    services,
		})
	}
	return DeploymentHistory{App: history.App, Versions: versions}
}

func ProjectDatabases(
	snapshot minibase.Snapshot,
) (DatabaseCollection, error) {
	if snapshot.CollectedAt.IsZero() || len(snapshot.Databases) > 500 {
		return DatabaseCollection{}, fmt.Errorf("invalid database snapshot")
	}
	databases := make([]Database, 0, len(snapshot.Databases))
	for _, database := range snapshot.Databases {
		if !minibase.ValidDatabaseID(database.ID) ||
			!safeText(database.DisplayName, 256, false) ||
			!safeText(database.Status, 64, false) ||
			database.SizeBytes < 0 ||
			database.Connections < 0 ||
			database.ActiveConnections < 0 ||
			database.IdleConnections < 0 ||
			database.Transactions.Commits < 0 ||
			database.Transactions.Rollbacks < 0 ||
			database.Cache.BlockReads < 0 ||
			database.Cache.BlockHits < 0 ||
			database.Rows.Inserted < 0 ||
			database.Rows.Updated < 0 ||
			database.Rows.Deleted < 0 ||
			database.BackupCount < 0 ||
			database.BackupBytes < 0 {

			return DatabaseCollection{}, fmt.Errorf("invalid database snapshot")
		}
		latestBackupAt := cloneTime(database.LatestBackupAt)
		if latestBackupAt != nil && latestBackupAt.IsZero() {
			return DatabaseCollection{}, fmt.Errorf("invalid database snapshot")
		}
		if database.BackupAgeSeconds != nil &&
			*database.BackupAgeSeconds < 0 {

			return DatabaseCollection{}, fmt.Errorf("invalid database snapshot")
		}
		deployments := make(
			[]DatabaseDeployment,
			0,
			len(database.Attachments),
		)
		for _, attachment := range database.Attachments {
			if attachment.ConsumerType != "minideploy" ||
				attachment.BindingName != "primary" ||
				!minideploy.ValidApplicationID(attachment.ConsumerRef) {

				return DatabaseCollection{}, fmt.Errorf(
					"invalid database attachment",
				)
			}
			deployments = append(deployments, DatabaseDeployment{
				App:         attachment.ConsumerRef,
				BindingName: attachment.BindingName,
			})
		}
		databases = append(databases, Database{
			ID:                database.ID,
			DisplayName:       database.DisplayName,
			Status:            database.Status,
			SizeBytes:         database.SizeBytes,
			Connections:       database.Connections,
			ActiveConnections: database.ActiveConnections,
			IdleConnections:   database.IdleConnections,
			Commits:           database.Transactions.Commits,
			Rollbacks:         database.Transactions.Rollbacks,
			BlockReads:        database.Cache.BlockReads,
			BlockHits:         database.Cache.BlockHits,
			RowsInserted:      database.Rows.Inserted,
			RowsUpdated:       database.Rows.Updated,
			RowsDeleted:       database.Rows.Deleted,
			BackupCount:       database.BackupCount,
			BackupBytes:       database.BackupBytes,
			LatestBackupAt:    latestBackupAt,
			BackupAgeSeconds:  cloneFloat(database.BackupAgeSeconds),
			Deployments:       deployments,
		})
	}
	return DatabaseCollection{
		CollectedAt: snapshot.CollectedAt.UTC(),
		Databases:   databases,
	}, nil
}

func ProjectBackups(
	databaseID string,
	backups []minibase.Backup,
	limit int,
) BackupCollection {
	sort.SliceStable(backups, func(i, j int) bool {
		if backups[i].CreatedAt.Equal(backups[j].CreatedAt) {
			return backups[i].ID > backups[j].ID
		}
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})
	truncated := len(backups) > limit
	if truncated {
		backups = backups[:limit]
	}
	items := make([]Backup, 0, len(backups))
	for _, backup := range backups {
		items = append(items, Backup{
			ID:          backup.ID,
			DatabaseID:  backup.DatabaseID,
			Kind:        backup.Kind,
			Status:      backup.Status,
			SizeBytes:   backup.SizeBytes,
			CreatedAt:   backup.CreatedAt.UTC(),
			CompletedAt: cloneTime(backup.CompletedAt),
		})
	}
	return BackupCollection{
		DatabaseID: databaseID,
		Backups:    items,
		Limit:      limit,
		Truncated:  truncated,
	}
}

func ProjectHost(points []observability.HostPoint) []HostPoint {
	result := make([]HostPoint, 0, len(points))
	for _, point := range points {
		result = append(result, HostPoint{
			Timestamp:         point.Timestamp.UTC(),
			SampleCount:       point.SampleCount,
			CPUAverage:        cloneFloat(point.CPUAverage),
			CPUMaximum:        cloneFloat(point.CPUMaximum),
			MemoryUsedAverage: point.MemoryUsedAverage,
			MemoryUsedMaximum: point.MemoryUsedMaximum,
			MemoryTotal:       point.MemoryTotal,
			Load1Average:      point.Load1Average,
			Load5Average:      point.Load5Average,
			Load15Average:     point.Load15Average,
			DiskUsedAverage:   point.DiskUsedAverage,
			DiskTotal:         point.DiskTotal,
			DiskReadAverage:   cloneFloat(point.DiskReadAverage),
			DiskReadMaximum:   cloneFloat(point.DiskReadMaximum),
			DiskWriteAverage:  cloneFloat(point.DiskWriteAverage),
			DiskWriteMaximum:  cloneFloat(point.DiskWriteMaximum),
			NetworkRXAverage:  cloneFloat(point.NetworkRXAverage),
			NetworkRXMaximum:  cloneFloat(point.NetworkRXMaximum),
			NetworkTXAverage:  cloneFloat(point.NetworkTXAverage),
			NetworkTXMaximum:  cloneFloat(point.NetworkTXMaximum),
		})
	}
	return result
}

func ProjectTemperatures(
	points []observability.TemperaturePoint,
) []TemperaturePoint {
	result := make([]TemperaturePoint, 0, len(points))
	for _, point := range points {
		result = append(result, TemperaturePoint{
			BucketStart: point.BucketStart.UTC(),
			BucketEnd:   point.BucketEnd.UTC(),
			SampleCount: point.SampleCount,
			MinCelsius:  point.MinCelsius,
			AvgCelsius:  point.AvgCelsius,
			MaxCelsius:  point.MaxCelsius,
			PeakAt:      point.PeakAt.UTC(),
		})
	}
	return result
}

func ProjectApplicationSummaries(
	items []observability.ApplicationSummary,
) []ApplicationSummary {
	result := make([]ApplicationSummary, 0, len(items))
	for _, item := range items {
		result = append(result, ApplicationSummary{
			ID:             item.ID,
			Name:           item.Name,
			LatestStatus:   item.LatestStatus,
			RestartCount:   item.RestartCount,
			LastObservedAt: item.LastObservedAt.UTC(),
		})
	}
	return result
}

func ProjectApplicationPoints(
	points []observability.ApplicationPoint,
) []ApplicationPoint {
	result := make([]ApplicationPoint, 0, len(points))
	for _, point := range points {
		result = append(result, ApplicationPoint{
			Timestamp:          point.Timestamp.UTC(),
			SampleCount:        point.SampleCount,
			CPUAverage:         point.CPUAverage,
			CPUMaximum:         point.CPUMaximum,
			MemoryUsedAverage:  point.MemoryUsedAverage,
			MemoryUsedMaximum:  point.MemoryUsedMaximum,
			MemoryLimitAverage: point.MemoryLimitAverage,
			NetworkRXAverage:   cloneFloat(point.NetworkRXAverage),
			NetworkRXMaximum:   cloneFloat(point.NetworkRXMaximum),
			NetworkTXAverage:   cloneFloat(point.NetworkTXAverage),
			NetworkTXMaximum:   cloneFloat(point.NetworkTXMaximum),
			Status:             point.Status,
			RestartCount:       point.RestartCount,
		})
	}
	return result
}

func ProjectServices(
	series []observability.ServiceSeries,
) []ServiceSeries {
	result := make([]ServiceSeries, 0, len(series))
	for _, item := range series {
		points := make([]ServicePoint, 0, len(item.Points))
		for _, point := range item.Points {
			points = append(points, ServicePoint{
				Timestamp:   point.Timestamp.UTC(),
				SampleCount: point.SampleCount,
				Available:   point.Available,
				Status:      point.Status,
			})
		}
		result = append(result, ServiceSeries{
			ID:     item.ID,
			Name:   item.Name,
			Points: points,
		})
	}
	return result
}

func ProjectEvents(events []observability.Event) []Event {
	result := make([]Event, 0, len(events))
	for _, event := range events {
		result = append(result, Event{
			ID:           event.ID,
			Source:       event.Source,
			Type:         event.Type,
			ResourceType: event.ResourceType,
			ResourceID:   event.ResourceID,
			ResourceName: event.ResourceName,
			OccurredAt:   event.OccurredAt.UTC(),
			Summary:      event.Summary,
		})
	}
	return result
}

func ProjectActivity(
	legacy []history.ActivityEvent,
	recoveries []observability.RecoveryIncident,
	limit int,
) ActivityResponse {
	events := make(
		[]ActivityEvent,
		0,
		len(legacy)+len(recoveries),
	)
	for _, event := range legacy {
		events = append(events, ActivityEvent{
			OccurredAt: event.OccurredAt.UTC(),
			Source:     event.Source,
			Kind:       event.Kind,
			Severity:   event.Severity,
			Subject:    event.Subject,
			Message:    event.Message,
		})
	}
	for _, recovery := range recoveries {
		if recovery.EventID == "" ||
			recovery.LastKnownAliveAt.IsZero() ||
			recovery.RecoveredAt.IsZero() ||
			recovery.DowntimeSeconds < 0 ||
			recovery.Status != "recovered" {

			continue
		}
		events = append(events, ActivityEvent{
			EventID:    recovery.EventID,
			OccurredAt: recovery.RecoveredAt.UTC(),
			Source:     "reactorlab",
			Kind:       "unexpected_shutdown_recovery",
			Severity:   "warning",
			Subject:    "Unexpected shutdown detected",
			Message:    "Dell host recovered after an unexpected shutdown.",
			Incident: &ActivityIncident{
				LastKnownAliveAt: recovery.LastKnownAliveAt.UTC(),
				RecoveredAt:      recovery.RecoveredAt.UTC(),
				DowntimeSeconds:  recovery.DowntimeSeconds,
				Status:           recovery.Status,
			},
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].EventID > events[j].EventID
		}
		return events[i].OccurredAt.After(events[j].OccurredAt)
	})
	if len(events) > limit {
		events = events[:limit]
	}
	return ActivityResponse{Limit: limit, Events: events}
}

func FindDeployment(
	collection DeploymentCollection,
	app string,
) (Deployment, bool) {
	for _, deployment := range collection.Deployments {
		if deployment.App == app {
			return deployment, true
		}
	}
	return Deployment{}, false
}

func FindDatabase(
	collection DatabaseCollection,
	id string,
) (Database, bool) {
	for _, database := range collection.Databases {
		if database.ID == id {
			return database, true
		}
	}
	return Database{}, false
}

func projectSource(source *minideploy.Source) *Source {
	if source == nil {
		return nil
	}
	return &Source{
		Provider:     source.Provider,
		Repository:   source.Repository,
		Branch:       source.Branch,
		RequestedRef: source.RequestedRef,
		CommitSHA:    source.CommitSHA,
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func safeText(value string, maximum int, allowEmpty bool) bool {
	if (!allowEmpty && strings.TrimSpace(value) == "") ||
		len(value) > maximum {

		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
