package intelligence

import "time"

type Section[T any] struct {
	Available bool   `json:"available"`
	Data      *T     `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Overview struct {
	CollectedAt   time.Time                          `json:"collectedAt"`
	System        Section[SystemSnapshot]            `json:"system"`
	Recovery      Section[Recovery]                  `json:"recovery"`
	Deployments   Section[DeploymentCollection]      `json:"deployments"`
	Databases     Section[DatabaseCollection]        `json:"databases"`
	Observability Section[ObservabilityCurrentState] `json:"observability"`
}

type CPUState struct {
	UsagePercent float64 `json:"usagePercent"`
	LogicalCores int     `json:"logicalCores"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
}

type MemoryState struct {
	TotalBytes       uint64  `json:"totalBytes"`
	UsedBytes        uint64  `json:"usedBytes"`
	AvailableBytes   uint64  `json:"availableBytes"`
	UsagePercent     float64 `json:"usagePercent"`
	SwapTotalBytes   uint64  `json:"swapTotalBytes"`
	SwapUsedBytes    uint64  `json:"swapUsedBytes"`
	SwapUsagePercent float64 `json:"swapUsagePercent"`
}

type DiskState struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
}

type TemperatureState struct {
	Celsius float64 `json:"celsius"`
	Source  string  `json:"source"`
}

type NetworkState struct {
	Interface     string  `json:"interface"`
	RXBytes       uint64  `json:"rxBytes"`
	TXBytes       uint64  `json:"txBytes"`
	RXBytesPerSec float64 `json:"rxBytesPerSecond"`
	TXBytesPerSec float64 `json:"txBytesPerSecond"`
}

type BatteryState struct {
	Available   bool    `json:"available"`
	Percent     float64 `json:"percent"`
	ACAvailable bool    `json:"acAvailable"`
	ACConnected bool    `json:"acConnected"`
	Status      string  `json:"status"`
}

type ServiceState struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Active bool   `json:"active"`
}

type SystemSnapshot struct {
	CPU           CPUState         `json:"cpu"`
	Memory        MemoryState      `json:"memory"`
	Disk          DiskState        `json:"disk"`
	Temperature   TemperatureState `json:"temperature"`
	UptimeSeconds float64          `json:"uptimeSeconds"`
	Network       NetworkState     `json:"network"`
	Battery       BatteryState     `json:"battery"`
	Services      []ServiceState   `json:"services"`
	CollectedAt   time.Time        `json:"collectedAt"`
}

type ProtectionState struct {
	State            string                `json:"state"`
	HardwareWatchdog HardwareWatchdogState `json:"hardwareWatchdog"`
	RTC              RTCRecoveryState      `json:"rtc"`
}

type HardwareWatchdogState struct {
	State          string  `json:"state"`
	Identity       string  `json:"identity,omitempty"`
	TimeoutSeconds uint32  `json:"timeoutSeconds,omitempty"`
	BootStatus     *uint32 `json:"bootStatus,omitempty"`
}

type RTCRecoveryState struct {
	State  string     `json:"state"`
	WakeAt *time.Time `json:"wakeAt,omitempty"`
}

type RecoveryIncident struct {
	EventID          string    `json:"eventId"`
	LastKnownAliveAt time.Time `json:"lastKnownAliveAt"`
	RecoveredAt      time.Time `json:"recoveredAt"`
	DowntimeSeconds  int64     `json:"downtimeSeconds"`
	Status           string    `json:"status"`
}

type Recovery struct {
	Protection       ProtectionState    `json:"protection"`
	HistoryAvailable bool               `json:"historyAvailable"`
	LastIncident     *RecoveryIncident  `json:"lastIncident,omitempty"`
	RecentIncidents  []RecoveryIncident `json:"recentIncidents"`
}

type Source struct {
	Provider     string `json:"provider,omitempty"`
	Repository   string `json:"repository,omitempty"`
	Branch       string `json:"branch,omitempty"`
	RequestedRef string `json:"requestedRef,omitempty"`
	CommitSHA    string `json:"commitSha"`
}

type DeploymentService struct {
	Name     string `json:"name"`
	Strategy string `json:"strategy,omitempty"`
	Status   string `json:"status,omitempty"`
	ImageID  string `json:"imageId,omitempty"`
}

type DeploymentDatabase struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	BindingName string `json:"bindingName"`
}

type Deployment struct {
	App         string               `json:"app"`
	Strategy    string               `json:"strategy"`
	Status      string               `json:"status"`
	Source      *Source              `json:"source,omitempty"`
	ActivatedAt *time.Time           `json:"activatedAt,omitempty"`
	ImageID     string               `json:"imageId,omitempty"`
	Services    []DeploymentService  `json:"services"`
	Databases   []DeploymentDatabase `json:"databases"`
}

type DeploymentCollection struct {
	Deployments []Deployment `json:"deployments"`
}

type DeploymentVersion struct {
	App         string              `json:"app"`
	Strategy    string              `json:"strategy,omitempty"`
	Source      *Source             `json:"source,omitempty"`
	ActivatedAt *time.Time          `json:"activatedAt,omitempty"`
	ArchivedAt  time.Time           `json:"archivedAt"`
	ImageID     string              `json:"imageId,omitempty"`
	Services    []DeploymentService `json:"services"`
}

type DeploymentHistory struct {
	App      string              `json:"app"`
	Versions []DeploymentVersion `json:"versions"`
}

type DatabaseDeployment struct {
	App         string `json:"app"`
	BindingName string `json:"bindingName"`
}

type Database struct {
	ID                string               `json:"id"`
	DisplayName       string               `json:"displayName"`
	Status            string               `json:"status"`
	SizeBytes         int64                `json:"sizeBytes"`
	Connections       int64                `json:"connections"`
	ActiveConnections int64                `json:"activeConnections"`
	IdleConnections   int64                `json:"idleConnections"`
	Commits           int64                `json:"commits"`
	Rollbacks         int64                `json:"rollbacks"`
	BlockReads        int64                `json:"blockReads"`
	BlockHits         int64                `json:"blockHits"`
	RowsInserted      int64                `json:"rowsInserted"`
	RowsUpdated       int64                `json:"rowsUpdated"`
	RowsDeleted       int64                `json:"rowsDeleted"`
	BackupCount       int                  `json:"backupCount"`
	BackupBytes       int64                `json:"backupBytes"`
	LatestBackupAt    *time.Time           `json:"latestBackupAt,omitempty"`
	BackupAgeSeconds  *float64             `json:"backupAgeSeconds,omitempty"`
	Deployments       []DatabaseDeployment `json:"deployments"`
}

type DatabaseCollection struct {
	CollectedAt time.Time  `json:"collectedAt"`
	Databases   []Database `json:"databases"`
}

type Backup struct {
	ID          string     `json:"id"`
	DatabaseID  string     `json:"databaseId"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	SizeBytes   int64      `json:"sizeBytes"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type BackupCollection struct {
	DatabaseID string   `json:"databaseId"`
	Backups    []Backup `json:"backups"`
	Limit      int      `json:"limit"`
	Truncated  bool     `json:"truncated"`
}

type Window struct {
	Range         string    `json:"range,omitempty"`
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
	BucketSeconds float64   `json:"bucketSeconds"`
	MaxPoints     int       `json:"maxPoints"`
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

type TemperaturePoint struct {
	BucketStart time.Time `json:"bucketStart"`
	BucketEnd   time.Time `json:"bucketEnd"`
	SampleCount int       `json:"sampleCount"`
	MinCelsius  float64   `json:"minCelsius"`
	AvgCelsius  float64   `json:"avgCelsius"`
	MaxCelsius  float64   `json:"maxCelsius"`
	PeakAt      time.Time `json:"peakAt"`
}

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

type Event struct {
	ID           string    `json:"id"`
	Source       string    `json:"source"`
	Type         string    `json:"type"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceId,omitempty"`
	ResourceName string    `json:"resourceName,omitempty"`
	OccurredAt   time.Time `json:"occurredAt"`
	Summary      string    `json:"summary"`
}

type ObservabilityCurrentState struct {
	Applications []ApplicationSummary `json:"applications"`
	Services     []ServiceSeries      `json:"services"`
}

type HostResponse struct {
	Window Window      `json:"window"`
	Points []HostPoint `json:"points"`
}

type TemperatureResponse struct {
	Window Window             `json:"window"`
	Points []TemperaturePoint `json:"points"`
}

type ApplicationsResponse struct {
	Window       Window               `json:"window"`
	Applications []ApplicationSummary `json:"applications"`
}

type ApplicationResponse struct {
	Window Window             `json:"window"`
	ID     string             `json:"id"`
	Points []ApplicationPoint `json:"points"`
}

type ServicesResponse struct {
	Window   Window          `json:"window"`
	Services []ServiceSeries `json:"services"`
}

type EventsResponse struct {
	Window Window  `json:"window"`
	Limit  int     `json:"limit"`
	Events []Event `json:"events"`
}

type ActivityIncident struct {
	LastKnownAliveAt time.Time `json:"lastKnownAliveAt"`
	RecoveredAt      time.Time `json:"recoveredAt"`
	DowntimeSeconds  int64     `json:"downtimeSeconds"`
	Status           string    `json:"status"`
}

type ActivityEvent struct {
	EventID    string            `json:"eventId,omitempty"`
	OccurredAt time.Time         `json:"occurredAt"`
	Source     string            `json:"source"`
	Kind       string            `json:"kind"`
	Severity   string            `json:"severity"`
	Subject    string            `json:"subject"`
	Message    string            `json:"message"`
	Incident   *ActivityIncident `json:"incident,omitempty"`
}

type ActivityResponse struct {
	Limit  int             `json:"limit"`
	Events []ActivityEvent `json:"events"`
}
