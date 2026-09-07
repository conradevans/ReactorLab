package minideploy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "http://127.0.0.1:9000"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Snapshot struct {
	Deployments []Deployment `json:"deployments"`
	CollectedAt time.Time    `json:"collectedAt"`
}

type Deployment struct {
	App        string             `json:"app"`
	Strategy   string             `json:"strategy"`
	Status     string             `json:"status"`
	Containers []ContainerMetrics `json:"containers"`
}

type ContainerMetrics struct {
	Service          string  `json:"service"`
	Strategy         string  `json:"strategy"`
	Container        string  `json:"container"`
	State            string  `json:"state"`
	Health           string  `json:"health"`
	CPUPercent       float64 `json:"cpuPercent"`
	MemoryUsedBytes  uint64  `json:"memoryUsedBytes"`
	MemoryLimitBytes uint64  `json:"memoryLimitBytes"`
	MemoryPercent    float64 `json:"memoryPercent"`
	NetworkRXBytes   uint64  `json:"networkRxBytes"`
	NetworkTXBytes   uint64  `json:"networkTxBytes"`
	BlockReadBytes   uint64  `json:"blockReadBytes"`
	BlockWriteBytes  uint64  `json:"blockWriteBytes"`
	PIDs             int     `json:"pids"`
	WritableBytes    int64   `json:"writableBytes"`
	UptimeSeconds    float64 `json:"uptimeSeconds"`
	RestartCount     int     `json:"restartCount"`
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

func (c *Client) Deployments(ctx context.Context) (Snapshot, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/internal/reactorlab/deployments",
		nil,
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("build MiniDeploy observability request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Snapshot{}, fmt.Errorf("request MiniDeploy observability: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Snapshot{}, fmt.Errorf(
			"MiniDeploy observability returned status %d",
			resp.StatusCode,
		)
	}

	var snapshot Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode MiniDeploy observability: %w", err)
	}

	if snapshot.Deployments == nil {
		snapshot.Deployments = []Deployment{}
	}

	return snapshot, nil
}

func FindDeployment(snapshot Snapshot, app string) (Deployment, bool) {
	for _, deployment := range snapshot.Deployments {
		if deployment.App == app {
			return deployment, true
		}
	}
	return Deployment{}, false
}
