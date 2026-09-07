package minibase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "http://127.0.0.1:9100"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Snapshot struct {
	Databases   []Database      `json:"databases"`
	Postgres    PostgresMetrics `json:"postgres"`
	CollectedAt time.Time       `json:"collectedAt"`
}

type Database struct {
	ID                string       `json:"id"`
	DisplayName       string       `json:"displayName"`
	Status            string       `json:"status"`
	SizeBytes         int64        `json:"sizeBytes"`
	Connections       int64        `json:"connections"`
	ActiveConnections int64        `json:"activeConnections"`
	IdleConnections   int64        `json:"idleConnections"`
	Transactions      Transactions `json:"transactions"`
	Cache             CacheMetrics `json:"cache"`
	Rows              RowMetrics   `json:"rows"`
	BackupCount       int          `json:"backupCount"`
	BackupBytes       int64        `json:"backupBytes"`
	LatestBackupAt    *time.Time   `json:"latestBackupAt,omitempty"`
	BackupAgeSeconds  *float64     `json:"backupAgeSeconds,omitempty"`
}

type Transactions struct {
	Commits   int64 `json:"commits"`
	Rollbacks int64 `json:"rollbacks"`
}

type CacheMetrics struct {
	BlockReads int64 `json:"blockReads"`
	BlockHits  int64 `json:"blockHits"`
}

type RowMetrics struct {
	Inserted int64 `json:"inserted"`
	Updated  int64 `json:"updated"`
	Deleted  int64 `json:"deleted"`
}

type PostgresMetrics struct {
	State            string  `json:"state"`
	CPUPercent       float64 `json:"cpuPercent"`
	MemoryUsedBytes  int64   `json:"memoryUsedBytes"`
	MemoryLimitBytes int64   `json:"memoryLimitBytes"`
	MemoryPercent    float64 `json:"memoryPercent"`
	NetworkRXBytes   int64   `json:"networkRxBytes"`
	NetworkTXBytes   int64   `json:"networkTxBytes"`
	BlockReadBytes   int64   `json:"blockReadBytes"`
	BlockWriteBytes  int64   `json:"blockWriteBytes"`
	PIDs             int64   `json:"pids"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Databases(ctx context.Context) (Snapshot, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/internal/reactorlab/databases",
		nil,
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("build MiniBase observability request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Snapshot{}, fmt.Errorf("request MiniBase observability: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Snapshot{}, fmt.Errorf(
			"MiniBase observability returned status %d",
			resp.StatusCode,
		)
	}

	var snapshot Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode MiniBase observability: %w", err)
	}

	if snapshot.Databases == nil {
		snapshot.Databases = []Database{}
	}

	return snapshot, nil
}

func FindDatabase(snapshot Snapshot, id string) (Database, bool) {
	for _, database := range snapshot.Databases {
		if database.ID == id {
			return database, true
		}
	}
	return Database{}, false
}
