package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type QueryService struct {
	store *Store
}

func NewQueryService(store *Store) *QueryService {
	return &QueryService{store: store}
}

func (q *QueryService) QueryHost(ctx context.Context, window Range) ([]HostPoint, error) {
	if err := validateResolvedRange(window); err != nil {
		return nil, err
	}
	from, to, bucket := window.From.UTC().UnixMilli(), window.To.UTC().UnixMilli(), window.Bucket.Milliseconds()
	rows, err := q.store.db.QueryContext(ctx, `SELECT
		((collected_at_ms - ?) / ?) * ? + ? AS bucket_ms,
		COUNT(*), AVG(cpu_percent), MAX(cpu_percent),
		AVG(memory_used_bytes), MAX(memory_used_bytes), AVG(memory_total_bytes),
		AVG(load1), AVG(load5), AVG(load15),
		AVG(disk_used_bytes), AVG(disk_total_bytes),
		AVG(disk_read_bps), MAX(disk_read_bps), AVG(disk_write_bps), MAX(disk_write_bps),
		AVG(network_rx_bps), MAX(network_rx_bps), AVG(network_tx_bps), MAX(network_tx_bps)
	FROM host_samples
	WHERE collected_at_ms >= ? AND collected_at_ms < ?
	GROUP BY bucket_ms ORDER BY bucket_ms`, from, bucket, bucket, from, from, to)
	if err != nil {
		return nil, fmt.Errorf("query host observability: %w", err)
	}
	defer rows.Close()
	points := make([]HostPoint, 0)
	for rows.Next() {
		var timestamp int64
		var point HostPoint
		var cpuAvg, cpuMax, diskReadAvg, diskReadMax, diskWriteAvg, diskWriteMax sql.NullFloat64
		var networkRXAvg, networkRXMax, networkTXAvg, networkTXMax sql.NullFloat64
		if err := rows.Scan(&timestamp, &point.SampleCount, &cpuAvg, &cpuMax,
			&point.MemoryUsedAverage, &point.MemoryUsedMaximum, &point.MemoryTotal,
			&point.Load1Average, &point.Load5Average, &point.Load15Average,
			&point.DiskUsedAverage, &point.DiskTotal, &diskReadAvg, &diskReadMax,
			&diskWriteAvg, &diskWriteMax, &networkRXAvg, &networkRXMax,
			&networkTXAvg, &networkTXMax); err != nil {
			return nil, fmt.Errorf("scan host observability: %w", err)
		}
		point.Timestamp = time.UnixMilli(timestamp).UTC()
		point.CPUAverage, point.CPUMaximum = nullFloat(cpuAvg), nullFloat(cpuMax)
		point.DiskReadAverage, point.DiskReadMaximum = nullFloat(diskReadAvg), nullFloat(diskReadMax)
		point.DiskWriteAverage, point.DiskWriteMaximum = nullFloat(diskWriteAvg), nullFloat(diskWriteMax)
		point.NetworkRXAverage, point.NetworkRXMaximum = nullFloat(networkRXAvg), nullFloat(networkRXMax)
		point.NetworkTXAverage, point.NetworkTXMaximum = nullFloat(networkTXAvg), nullFloat(networkTXMax)
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate host observability: %w", err)
	}
	if len(points) > window.MaxPoints {
		return nil, fmt.Errorf("host query exceeded point bound")
	}
	return points, nil
}

func (q *QueryService) QueryTemperature(ctx context.Context, window Range) ([]TemperaturePoint, error) {
	if err := validateResolvedRange(window); err != nil {
		return nil, err
	}
	from, to, bucket := window.From.UTC().UnixMilli(), window.To.UTC().UnixMilli(), window.Bucket.Milliseconds()
	rows, err := q.store.db.QueryContext(ctx, `WITH raw AS (
		SELECT ((bucket_start_ms - ?) / ?) * ? + ? AS display_bucket_ms,
			bucket_end_ms, sample_count, min_celsius, avg_celsius, max_celsius, peak_at_ms
		FROM temperature_buckets
		WHERE bucket_start_ms >= ? AND bucket_start_ms < ?
	), ranked AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY display_bucket_ms ORDER BY max_celsius DESC, peak_at_ms ASC
		) AS peak_rank
		FROM raw
	)
	SELECT display_bucket_ms, MAX(bucket_end_ms), SUM(sample_count),
		MIN(min_celsius), SUM(avg_celsius * sample_count) / SUM(sample_count),
		MAX(max_celsius), MAX(CASE WHEN peak_rank = 1 THEN peak_at_ms END)
	FROM ranked GROUP BY display_bucket_ms ORDER BY display_bucket_ms`,
		from, bucket, bucket, from, from, to)
	if err != nil {
		return nil, fmt.Errorf("query temperature observability: %w", err)
	}
	defer rows.Close()
	points := make([]TemperaturePoint, 0)
	for rows.Next() {
		var start, end, peak int64
		var point TemperaturePoint
		if err := rows.Scan(&start, &end, &point.SampleCount, &point.MinCelsius,
			&point.AvgCelsius, &point.MaxCelsius, &peak); err != nil {
			return nil, fmt.Errorf("scan temperature observability: %w", err)
		}
		point.BucketStart = time.UnixMilli(start).UTC()
		point.BucketEnd = time.UnixMilli(end).UTC()
		point.PeakAt = time.UnixMilli(peak).UTC()
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate temperature observability: %w", err)
	}
	if len(points) > window.MaxPoints {
		return nil, fmt.Errorf("temperature query exceeded point bound")
	}
	return points, nil
}

func (q *QueryService) QueryApplications(ctx context.Context, window Range) ([]ApplicationSummary, error) {
	if err := validateResolvedRange(window); err != nil {
		return nil, err
	}
	rows, err := q.store.db.QueryContext(ctx, `SELECT s.app_id, s.app_name, s.status,
		s.restart_count, s.collected_at_ms FROM application_samples s
		JOIN (SELECT app_id, MAX(collected_at_ms) AS latest FROM application_samples
			WHERE collected_at_ms >= ? AND collected_at_ms < ? GROUP BY app_id) latest
		ON latest.app_id = s.app_id AND latest.latest = s.collected_at_ms
		ORDER BY s.app_name`, window.From.UTC().UnixMilli(), window.To.UTC().UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query application summaries: %w", err)
	}
	defer rows.Close()
	items := make([]ApplicationSummary, 0)
	for rows.Next() {
		var item ApplicationSummary
		var observed int64
		if err := rows.Scan(&item.ID, &item.Name, &item.LatestStatus, &item.RestartCount, &observed); err != nil {
			return nil, fmt.Errorf("scan application summary: %w", err)
		}
		item.LastObservedAt = time.UnixMilli(observed).UTC()
		items = append(items, item)
	}
	return items, rows.Err()
}

func (q *QueryService) QueryApplication(ctx context.Context, appID string, window Range) ([]ApplicationPoint, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, ErrInvalidRange
	}
	if err := validateResolvedRange(window); err != nil {
		return nil, err
	}
	from, to, bucket := window.From.UTC().UnixMilli(), window.To.UTC().UnixMilli(), window.Bucket.Milliseconds()
	rows, err := q.store.db.QueryContext(ctx, `SELECT
		((collected_at_ms - ?) / ?) * ? + ? AS bucket_ms, COUNT(*),
		AVG(cpu_percent), MAX(cpu_percent), AVG(memory_used_bytes), MAX(memory_used_bytes),
		AVG(memory_limit_bytes), AVG(network_rx_bps), MAX(network_rx_bps),
		AVG(network_tx_bps), MAX(network_tx_bps), CASE WHEN MIN(CASE WHEN status = 'healthy' THEN 1 ELSE 0 END) = 1 THEN 'healthy' ELSE 'unavailable' END, MAX(restart_count)
		FROM application_samples WHERE app_id = ? AND collected_at_ms >= ? AND collected_at_ms < ?
		GROUP BY bucket_ms ORDER BY bucket_ms`, from, bucket, bucket, from, appID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query application observability: %w", err)
	}
	defer rows.Close()
	points := make([]ApplicationPoint, 0)
	for rows.Next() {
		var timestamp int64
		var rxAvg, rxMax, txAvg, txMax sql.NullFloat64
		var point ApplicationPoint
		if err := rows.Scan(&timestamp, &point.SampleCount, &point.CPUAverage,
			&point.CPUMaximum, &point.MemoryUsedAverage, &point.MemoryUsedMaximum,
			&point.MemoryLimitAverage, &rxAvg, &rxMax, &txAvg, &txMax,
			&point.Status, &point.RestartCount); err != nil {
			return nil, fmt.Errorf("scan application observability: %w", err)
		}
		point.Timestamp = time.UnixMilli(timestamp).UTC()
		point.NetworkRXAverage, point.NetworkRXMaximum = nullFloat(rxAvg), nullFloat(rxMax)
		point.NetworkTXAverage, point.NetworkTXMaximum = nullFloat(txAvg), nullFloat(txMax)
		points = append(points, point)
	}
	if len(points) > window.MaxPoints {
		return nil, fmt.Errorf("application query exceeded point bound")
	}
	return points, rows.Err()
}

func (q *QueryService) QueryServices(ctx context.Context, window Range) ([]ServiceSeries, error) {
	if err := validateResolvedRange(window); err != nil {
		return nil, err
	}
	from, to, bucket := window.From.UTC().UnixMilli(), window.To.UTC().UnixMilli(), window.Bucket.Milliseconds()
	rows, err := q.store.db.QueryContext(ctx, `SELECT service_id, MAX(service_name),
		((collected_at_ms - ?) / ?) * ? + ? AS display_bucket_ms,
		COUNT(*), MIN(available),
		CASE WHEN MIN(available) = 1 THEN 'healthy' ELSE 'unavailable' END
	FROM service_samples
	WHERE collected_at_ms >= ? AND collected_at_ms < ?
	GROUP BY service_id, display_bucket_ms
	ORDER BY service_id, display_bucket_ms`,
		from, bucket, bucket, from, from, to)
	if err != nil {
		return nil, fmt.Errorf("query service observability: %w", err)
	}
	defer rows.Close()
	series := make([]ServiceSeries, 0)
	for rows.Next() {
		var id, name, status string
		var timestamp int64
		var sampleCount, available int
		if err := rows.Scan(&id, &name, &timestamp, &sampleCount, &available, &status); err != nil {
			return nil, fmt.Errorf("scan service observability: %w", err)
		}
		if len(series) == 0 || series[len(series)-1].ID != id {
			series = append(series, ServiceSeries{ID: id, Name: name, Points: []ServicePoint{}})
		}
		current := &series[len(series)-1]
		current.Points = append(current.Points, ServicePoint{
			Timestamp: time.UnixMilli(timestamp).UTC(), SampleCount: sampleCount,
			Available: available == 1, Status: status,
		})
		if len(current.Points) > window.MaxPoints {
			return nil, fmt.Errorf("service query exceeded point bound")
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service observability: %w", err)
	}
	return series, nil
}

func (q *QueryService) QueryEvents(ctx context.Context, from, to time.Time, limit int) ([]Event, error) {
	if from.IsZero() || to.IsZero() || !from.Before(to) || limit < 1 || limit > 500 {
		return nil, ErrInvalidRange
	}
	rows, err := q.store.db.QueryContext(ctx, `SELECT event_id, source, source_event_id,
		event_type, resource_type, resource_id, resource_name, occurred_at_ms,
		summary, details_json FROM events WHERE occurred_at_ms >= ? AND occurred_at_ms < ?
		ORDER BY occurred_at_ms DESC LIMIT ?`, from.UTC().UnixMilli(), to.UTC().UnixMilli(), limit)
	if err != nil {
		return nil, fmt.Errorf("query observability events: %w", err)
	}
	defer rows.Close()
	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var occurred int64
		var details string
		if err := rows.Scan(&event.ID, &event.Source, &event.SourceEventID,
			&event.Type, &event.ResourceType, &event.ResourceID, &event.ResourceName,
			&occurred, &event.Summary, &details); err != nil {
			return nil, fmt.Errorf("scan observability event: %w", err)
		}
		event.OccurredAt = time.UnixMilli(occurred).UTC()
		if details != "{}" {
			if err := json.Unmarshal([]byte(details), &event.Details); err != nil {
				return nil, fmt.Errorf("decode observability event details: %w", err)
			}
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (q *QueryService) LatestRecoveryIncident(ctx context.Context) (*RecoveryIncident, error) {
	row := q.store.db.QueryRowContext(ctx, `SELECT event_id, details_json
		FROM events WHERE event_type = ?
		ORDER BY occurred_at_ms DESC, event_id DESC LIMIT 1`, recoveryEventType)
	var eventID, detailsJSON string
	if err := row.Scan(&eventID, &detailsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest recovery incident: %w", err)
	}
	var details struct {
		LastKnownAliveAt time.Time `json:"lastKnownAliveAt"`
		RecoveredAt      time.Time `json:"recoveredAt"`
		DowntimeSeconds  int64     `json:"downtimeSeconds"`
		Status           string    `json:"status"`
		PreviousBootID   string    `json:"previousBootId"`
		RecoveryBootID   string    `json:"recoveryBootId"`
	}
	if err := json.Unmarshal([]byte(detailsJSON), &details); err != nil {
		return nil, fmt.Errorf("decode latest recovery incident: %w", err)
	}
	if eventID == "" || details.LastKnownAliveAt.IsZero() || details.RecoveredAt.IsZero() ||
		details.DowntimeSeconds < 0 || details.Status != recoveryIncidentStatus ||
		strings.TrimSpace(details.PreviousBootID) == "" ||
		strings.TrimSpace(details.RecoveryBootID) == "" {
		return nil, fmt.Errorf("decode latest recovery incident: invalid recovery details")
	}
	return &RecoveryIncident{
		EventID:          eventID,
		LastKnownAliveAt: details.LastKnownAliveAt.UTC(),
		RecoveredAt:      details.RecoveredAt.UTC(),
		DowntimeSeconds:  details.DowntimeSeconds,
		Status:           details.Status,
		PreviousBootID:   details.PreviousBootID,
		RecoveryBootID:   details.RecoveryBootID,
	}, nil
}

func recoveryIncidentFromJSON(eventID, detailsJSON string) (*RecoveryIncident, error) {
	var details struct {
		LastKnownAliveAt time.Time `json:"lastKnownAliveAt"`
		RecoveredAt      time.Time `json:"recoveredAt"`
		DowntimeSeconds  int64     `json:"downtimeSeconds"`
		Status           string    `json:"status"`
		PreviousBootID   string    `json:"previousBootId"`
		RecoveryBootID   string    `json:"recoveryBootId"`
	}
	if err := json.Unmarshal([]byte(detailsJSON), &details); err != nil {
		return nil, fmt.Errorf("decode recovery incident: %w", err)
	}
	if eventID == "" || details.LastKnownAliveAt.IsZero() || details.RecoveredAt.IsZero() ||
		details.DowntimeSeconds < 0 || details.Status != recoveryIncidentStatus ||
		strings.TrimSpace(details.PreviousBootID) == "" ||
		strings.TrimSpace(details.RecoveryBootID) == "" {
		return nil, fmt.Errorf("decode recovery incident: invalid recovery details")
	}
	return &RecoveryIncident{
		EventID: eventID, LastKnownAliveAt: details.LastKnownAliveAt.UTC(),
		RecoveredAt: details.RecoveredAt.UTC(), DowntimeSeconds: details.DowntimeSeconds,
		Status: details.Status, PreviousBootID: details.PreviousBootID,
		RecoveryBootID: details.RecoveryBootID,
	}, nil
}

func (q *QueryService) ListRecoveryIncidents(ctx context.Context, limit int) ([]RecoveryIncident, error) {
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidRange
	}
	rows, err := q.store.db.QueryContext(ctx, `SELECT event_id, details_json
		FROM events WHERE event_type = ? AND source = ? AND resource_type = ?
		ORDER BY occurred_at_ms DESC, event_id DESC LIMIT ?`,
		recoveryEventType, "reactorlab", "host", limit)
	if err != nil {
		return nil, fmt.Errorf("query recovery incidents: %w", err)
	}
	defer rows.Close()
	incidents := make([]RecoveryIncident, 0)
	for rows.Next() {
		var eventID, detailsJSON string
		if err := rows.Scan(&eventID, &detailsJSON); err != nil {
			return nil, fmt.Errorf("scan recovery incident: %w", err)
		}
		incident, err := recoveryIncidentFromJSON(eventID, detailsJSON)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, *incident)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recovery incidents: %w", err)
	}
	return incidents, nil
}

func (q *QueryService) RecoveryIncidentByID(ctx context.Context, eventID string) (*RecoveryIncident, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, ErrInvalidRange
	}
	row := q.store.db.QueryRowContext(ctx, `SELECT event_id, details_json
		FROM events WHERE event_id = ? AND event_type = ? AND source = ? AND resource_type = ?`,
		eventID, recoveryEventType, "reactorlab", "host")
	var storedEventID, detailsJSON string
	if err := row.Scan(&storedEventID, &detailsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query recovery incident by ID: %w", err)
	}
	return recoveryIncidentFromJSON(storedEventID, detailsJSON)
}

func validateResolvedRange(window Range) error {
	if err := ValidateWindow(window.From, window.To, window.MaxPoints); err != nil {
		return err
	}
	duration := window.To.Sub(window.From)
	if window.Bucket <= 0 || window.Bucket > duration {
		return ErrInvalidRange
	}
	bucketCount := int64(duration / window.Bucket)
	if duration%window.Bucket != 0 {
		bucketCount++
	}
	if bucketCount > int64(window.MaxPoints) || bucketCount > int64(MaxDisplayPoints) {
		return ErrInvalidRange
	}
	return nil
}

func nullFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}
