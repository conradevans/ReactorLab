package minibase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "http://127.0.0.1:9100"

const maxDatabaseResponseBytes = 2 << 20

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type GuestSummary struct {
	Total   int `json:"total"`
	Showing int `json:"showing"`
	Hidden  int `json:"hidden"`
}

type GuestDatabase struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
}

type GuestDatabases struct {
	Summary   GuestSummary    `json:"summary"`
	Databases []GuestDatabase `json:"databases"`
}

type guestDatabasesEnvelope struct {
	Summary   *GuestSummary   `json:"summary"`
	Databases []GuestDatabase `json:"databases"`
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
	Attachments       []Attachment `json:"attachments"`
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

type Attachment struct {
	ConsumerType string `json:"consumerType"`
	ConsumerRef  string `json:"consumerRef"`
	BindingName  string `json:"bindingName"`
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

type Activity struct {
	DatabaseID          string    `json:"databaseId,omitempty"`
	DatabaseDisplayName string    `json:"databaseDisplayName,omitempty"`
	Type                string    `json:"type"`
	Outcome             string    `json:"outcome"`
	Source              string    `json:"source"`
	Detail              string    `json:"detail"`
	CreatedAt           time.Time `json:"createdAt"`
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
	return c.databases(ctx, false)
}

// ValidatedDatabases reads the existing ReactorLab observability endpoint with
// the stricter field-presence and JSON-boundary checks required by the MiniAI
// service contract. Databases retains its legacy null-list normalization for
// the existing administrator API.
func (c *Client) ValidatedDatabases(
	ctx context.Context,
) (Snapshot, error) {
	return c.databases(ctx, true)
}

func (c *Client) databases(
	ctx context.Context,
	strict bool,
) (Snapshot, error) {
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

	if !strict {
		var snapshot Snapshot
		if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
			return Snapshot{}, fmt.Errorf(
				"decode MiniBase observability: %w",
				err,
			)
		}
		if snapshot.Databases == nil {
			snapshot.Databases = []Database{}
		}
		for index := range snapshot.Databases {
			if snapshot.Databases[index].Attachments == nil {
				snapshot.Databases[index].Attachments = []Attachment{}
			}
		}
		return snapshot, nil
	}

	var envelope struct {
		Databases   json.RawMessage `json:"databases"`
		Postgres    json.RawMessage `json:"postgres"`
		CollectedAt time.Time       `json:"collectedAt"`
	}
	if err := decodeBoundedJSON(
		resp.Body,
		maxDatabaseResponseBytes,
		&envelope,
	); err != nil {
		return Snapshot{}, fmt.Errorf("decode MiniBase observability: %w", err)
	}
	if len(envelope.Databases) == 0 ||
		string(envelope.Databases) == "null" ||
		len(envelope.Postgres) == 0 ||
		string(envelope.Postgres) == "null" ||
		envelope.CollectedAt.IsZero() {

		return Snapshot{}, fmt.Errorf(
			"validate MiniBase observability: missing response fields",
		)
	}

	snapshot := Snapshot{CollectedAt: envelope.CollectedAt}
	if len(envelope.Databases) > 0 &&
		string(envelope.Databases) != "null" {

		if err := json.Unmarshal(
			envelope.Databases,
			&snapshot.Databases,
		); err != nil {
			return Snapshot{}, fmt.Errorf(
				"decode MiniBase observability databases: %w",
				err,
			)
		}
	}
	if len(envelope.Postgres) > 0 &&
		string(envelope.Postgres) != "null" {

		if err := json.Unmarshal(
			envelope.Postgres,
			&snapshot.Postgres,
		); err != nil {
			return Snapshot{}, fmt.Errorf(
				"decode MiniBase observability postgres: %w",
				err,
			)
		}
	}

	if snapshot.Databases == nil {
		snapshot.Databases = []Database{}
	}
	for index := range snapshot.Databases {
		if snapshot.Databases[index].Attachments == nil {
			snapshot.Databases[index].Attachments = []Attachment{}
		}
	}

	return snapshot, nil
}

func (c *Client) Activity(ctx context.Context) ([]Activity, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/v1/activity",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build MiniBase activity request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request MiniBase activity: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MiniBase activity returned status %d", resp.StatusCode)
	}
	var events []Activity
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&events); err != nil {
		return nil, fmt.Errorf("decode MiniBase activity: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("decode MiniBase activity: %w", err)
	}
	if events == nil {
		events = []Activity{}
	}
	for _, event := range events {
		if event.CreatedAt.IsZero() || strings.TrimSpace(event.Type) == "" ||
			strings.TrimSpace(event.Outcome) == "" || strings.TrimSpace(event.Source) == "" ||
			strings.TrimSpace(event.Detail) == "" {
			return nil, fmt.Errorf("validate MiniBase activity: invalid event")
		}
	}
	return events, nil
}

func (c *Client) GuestDatabases(ctx context.Context) (GuestDatabases, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/v1/guest/databases",
		nil,
	)
	if err != nil {
		return GuestDatabases{}, fmt.Errorf("build MiniBase Guest request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GuestDatabases{}, fmt.Errorf("request MiniBase Guest API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GuestDatabases{}, fmt.Errorf(
			"MiniBase Guest API returned status %d",
			resp.StatusCode,
		)
	}

	var envelope guestDatabasesEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return GuestDatabases{}, fmt.Errorf("decode MiniBase Guest API: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return GuestDatabases{}, fmt.Errorf("decode MiniBase Guest API: %w", err)
	}
	if envelope.Summary == nil || envelope.Databases == nil {
		return GuestDatabases{}, fmt.Errorf("validate MiniBase Guest API: missing response fields")
	}
	if err := validateGuestSummary(*envelope.Summary, len(envelope.Databases)); err != nil {
		return GuestDatabases{}, fmt.Errorf("validate MiniBase Guest API: %w", err)
	}
	for _, database := range envelope.Databases {
		if err := validateGuestDatabase(database); err != nil {
			return GuestDatabases{}, fmt.Errorf("validate MiniBase Guest API: %w", err)
		}
	}

	return GuestDatabases{
		Summary:   *envelope.Summary,
		Databases: envelope.Databases,
	}, nil
}

func validateGuestSummary(summary GuestSummary, itemCount int) error {
	if summary.Total < 0 || summary.Showing < 0 || summary.Hidden < 0 {
		return fmt.Errorf("summary counts must be non-negative")
	}
	if summary.Total != summary.Showing+summary.Hidden {
		return fmt.Errorf("summary total does not equal showing plus hidden")
	}
	if summary.Showing != itemCount {
		return fmt.Errorf("summary showing does not equal returned database count")
	}
	return nil
}

func validateGuestDatabase(database GuestDatabase) error {
	if strings.TrimSpace(database.ID) == "" ||
		strings.TrimSpace(database.DisplayName) == "" ||
		strings.TrimSpace(database.Status) == "" {
		return fmt.Errorf("database fields must be non-empty")
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func decodeBoundedJSON(
	reader io.Reader,
	maximum int64,
	target any,
) error {
	payload, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return err
	}
	if int64(len(payload)) > maximum {
		return fmt.Errorf("response exceeds %d bytes", maximum)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func FindDatabase(snapshot Snapshot, id string) (Database, bool) {
	for _, database := range snapshot.Databases {
		if database.ID == id {
			return database, true
		}
	}
	return Database{}, false
}
