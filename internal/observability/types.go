package observability

import (
	"context"
	"time"
)

const (
	SchemaVersion    = 3
	MetricRetention  = 7 * 24 * time.Hour
	MaxDisplayPoints = 720
)

type HostSample struct {
	CollectedAt      time.Time
	CPUPercent       *float64
	MemoryUsedBytes  uint64
	MemoryTotalBytes uint64
	Load1            float64
	Load5            float64
	Load15           float64
	DiskUsedBytes    uint64
	DiskTotalBytes   uint64
	DiskReadBPS      *float64
	DiskWriteBPS     *float64
	NetworkRXBPS     *float64
	NetworkTXBPS     *float64
	UptimeSeconds    float64
	BootID           string
}

type TemperatureBucket struct {
	BucketStart time.Time `json:"bucketStart"`
	BucketEnd   time.Time `json:"bucketEnd"`
	SampleCount int       `json:"sampleCount"`
	MinCelsius  float64   `json:"minCelsius"`
	AvgCelsius  float64   `json:"avgCelsius"`
	MaxCelsius  float64   `json:"maxCelsius"`
	PeakAt      time.Time `json:"peakAt"`
}

type ApplicationSample struct {
	CollectedAt      time.Time
	AppID            string
	Name             string
	Status           string
	CPUPercent       float64
	MemoryUsedBytes  uint64
	MemoryLimitBytes uint64
	NetworkRXBPS     *float64
	NetworkTXBPS     *float64
	RestartCount     int
}

type ServiceSample struct {
	CollectedAt     time.Time
	ServiceID       string
	Name            string
	Available       bool
	Status          string
	RestartIdentity string
}

type Event struct {
	ID            string         `json:"id"`
	Source        string         `json:"source"`
	SourceEventID string         `json:"sourceEventId"`
	Type          string         `json:"type"`
	ResourceType  string         `json:"resourceType"`
	ResourceID    string         `json:"resourceId,omitempty"`
	ResourceName  string         `json:"resourceName,omitempty"`
	OccurredAt    time.Time      `json:"occurredAt"`
	Summary       string         `json:"summary"`
	Details       map[string]any `json:"details,omitempty"`
}

type RecoveryIncident struct {
	EventID          string    `json:"eventId"`
	LastKnownAliveAt time.Time `json:"lastKnownAliveAt"`
	RecoveredAt      time.Time `json:"recoveredAt"`
	DowntimeSeconds  int64     `json:"downtimeSeconds"`
	Status           string    `json:"status"`
	PreviousBootID   string    `json:"previousBootId"`
	RecoveryBootID   string    `json:"recoveryBootId"`
}

// RecoveryIncidentWrite keeps recovery incidents canonical in the events table
// while making notification intent explicit for live versus historical writes.
type RecoveryIncidentWrite struct {
	Event  Event
	Notify bool
}

type AlertRule struct {
	ID              string    `json:"id"`
	Scope           string    `json:"scope"`
	ResourceID      string    `json:"resourceId,omitempty"`
	Metric          string    `json:"metric"`
	Operator        string    `json:"operator"`
	Threshold       float64   `json:"threshold"`
	DurationSeconds int       `json:"durationSeconds"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type AlertIncident struct {
	ID             string     `json:"id"`
	RuleID         string     `json:"ruleId"`
	ResourceType   string     `json:"resourceType"`
	ResourceID     string     `json:"resourceId,omitempty"`
	OpenedAt       time.Time  `json:"openedAt"`
	LastObservedAt time.Time  `json:"lastObservedAt"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	State          string     `json:"state"`
}

type Batch struct {
	Hosts        []HostSample
	Temperatures []TemperatureBucket
	Applications []ApplicationSample
	Services     []ServiceSample
	Events       []Event
	State        map[string]string
	Recoveries   []RecoveryIncidentWrite
}

type AlertRepository interface {
	PutAlertRule(context.Context, AlertRule) error
	ListAlertRules(context.Context) ([]AlertRule, error)
	PutAlertIncident(context.Context, AlertIncident) error
	ListAlertIncidents(context.Context, string) ([]AlertIncident, error)
}

type Range struct {
	Name      string
	From      time.Time
	To        time.Time
	Bucket    time.Duration
	MaxPoints int
}

type HostPoint struct {
	Timestamp         time.Time `json:"timestamp"`
	SampleCount       int       `json:"sampleCount"`
	CPUAverage        *float64  `json:"cpuAverage"`
	CPUMaximum        *float64  `json:"cpuMaximum"`
	MemoryUsedAverage float64   `json:"memoryUsedAverage"`
	MemoryUsedMaximum uint64    `json:"memoryUsedMaximum"`
	MemoryTotal       float64   `json:"memoryTotal"`
	Load1Average      float64   `json:"load1Average"`
	Load5Average      float64   `json:"load5Average"`
	Load15Average     float64   `json:"load15Average"`
	DiskUsedAverage   float64   `json:"diskUsedAverage"`
	DiskTotal         float64   `json:"diskTotal"`
	DiskReadAverage   *float64  `json:"diskReadAverage"`
	DiskReadMaximum   *float64  `json:"diskReadMaximum"`
	DiskWriteAverage  *float64  `json:"diskWriteAverage"`
	DiskWriteMaximum  *float64  `json:"diskWriteMaximum"`
	NetworkRXAverage  *float64  `json:"networkRxAverage"`
	NetworkRXMaximum  *float64  `json:"networkRxMaximum"`
	NetworkTXAverage  *float64  `json:"networkTxAverage"`
	NetworkTXMaximum  *float64  `json:"networkTxMaximum"`
}

type TemperaturePoint = TemperatureBucket

type ApplicationSummary struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	LatestStatus   string    `json:"latestStatus"`
	RestartCount   int       `json:"restartCount"`
	LastObservedAt time.Time `json:"lastObservedAt"`
}

type ApplicationPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	SampleCount        int       `json:"sampleCount"`
	CPUAverage         float64   `json:"cpuAverage"`
	CPUMaximum         float64   `json:"cpuMaximum"`
	MemoryUsedAverage  float64   `json:"memoryUsedAverage"`
	MemoryUsedMaximum  uint64    `json:"memoryUsedMaximum"`
	MemoryLimitAverage float64   `json:"memoryLimitAverage"`
	NetworkRXAverage   *float64  `json:"networkRxAverage"`
	NetworkRXMaximum   *float64  `json:"networkRxMaximum"`
	NetworkTXAverage   *float64  `json:"networkTxAverage"`
	NetworkTXMaximum   *float64  `json:"networkTxMaximum"`
	Status             string    `json:"status"`
	RestartCount       int       `json:"restartCount"`
}

type ServicePoint struct {
	Timestamp   time.Time `json:"timestamp"`
	SampleCount int       `json:"sampleCount"`
	Available   bool      `json:"available"`
	Status      string    `json:"status"`
}

type ServiceSeries struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Points []ServicePoint `json:"points"`
}

type HistoricalQuery interface {
	QueryHost(context.Context, Range) ([]HostPoint, error)
	QueryTemperature(context.Context, Range) ([]TemperaturePoint, error)
	QueryApplications(context.Context, Range) ([]ApplicationSummary, error)
	QueryApplication(context.Context, string, Range) ([]ApplicationPoint, error)
	QueryServices(context.Context, Range) ([]ServiceSeries, error)
	QueryEvents(context.Context, time.Time, time.Time, int) ([]Event, error)
	LatestRecoveryIncident(context.Context) (*RecoveryIncident, error)
	ListRecoveryIncidents(context.Context, int) ([]RecoveryIncident, error)
	RecoveryIncidentByID(context.Context, string) (*RecoveryIncident, error)
}
