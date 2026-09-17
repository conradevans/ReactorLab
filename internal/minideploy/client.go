package minideploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL      = "http://127.0.0.1:9000"
	DefaultGuestBaseURL = "http://127.0.0.1:9003"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type GuestSummary struct {
	Total   int `json:"total"`
	Showing int `json:"showing"`
	Hidden  int `json:"hidden"`
}

type GuestDeployment struct {
	App    string `json:"app"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

type GuestDeployments struct {
	Summary     GuestSummary      `json:"summary"`
	Deployments []GuestDeployment `json:"deployments"`
}

type guestDeploymentsEnvelope struct {
	Summary     *GuestSummary     `json:"summary"`
	Deployments []GuestDeployment `json:"deployments"`
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

func (c *Client) GuestDeployments(ctx context.Context) (GuestDeployments, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/guest/deployments",
		nil,
	)
	if err != nil {
		return GuestDeployments{}, fmt.Errorf("build MiniDeploy Guest request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GuestDeployments{}, fmt.Errorf("request MiniDeploy Guest API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GuestDeployments{}, fmt.Errorf(
			"MiniDeploy Guest API returned status %d",
			resp.StatusCode,
		)
	}

	var envelope guestDeploymentsEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return GuestDeployments{}, fmt.Errorf("decode MiniDeploy Guest API: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return GuestDeployments{}, fmt.Errorf("decode MiniDeploy Guest API: %w", err)
	}
	if envelope.Summary == nil || envelope.Deployments == nil {
		return GuestDeployments{}, fmt.Errorf("validate MiniDeploy Guest API: missing response fields")
	}
	if err := validateGuestSummary(*envelope.Summary, len(envelope.Deployments)); err != nil {
		return GuestDeployments{}, fmt.Errorf("validate MiniDeploy Guest API: %w", err)
	}
	for _, deployment := range envelope.Deployments {
		if err := validateGuestDeployment(deployment); err != nil {
			return GuestDeployments{}, fmt.Errorf("validate MiniDeploy Guest API: %w", err)
		}
	}

	return GuestDeployments{
		Summary:     *envelope.Summary,
		Deployments: envelope.Deployments,
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
		return fmt.Errorf("summary showing does not equal returned deployment count")
	}
	return nil
}

func validateGuestDeployment(deployment GuestDeployment) error {
	if strings.TrimSpace(deployment.App) == "" ||
		strings.TrimSpace(deployment.Status) == "" {
		return fmt.Errorf("deployment fields must be non-empty")
	}
	publicURL, err := url.Parse(deployment.URL)
	if err != nil || publicURL.Scheme != "https" ||
		publicURL.Host == "" || publicURL.Hostname() == "" {
		return fmt.Errorf("deployment URL must be an absolute HTTPS URL")
	}
	if publicURL.User != nil {
		return fmt.Errorf("deployment URL must not contain user information")
	}

	hostname := strings.TrimSuffix(
		strings.ToLower(publicURL.Hostname()),
		".",
	)
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") ||
		hostname == "local" || strings.HasSuffix(hostname, ".local") {
		return fmt.Errorf("deployment URL must use a public hostname")
	}
	if address, err := netip.ParseAddr(hostname); err == nil {
		address = address.Unmap()
		if address.IsLoopback() || address.IsUnspecified() ||
			address.IsPrivate() || address.IsLinkLocalUnicast() ||
			address.IsLinkLocalMulticast() {
			return fmt.Errorf("deployment URL must use a public address")
		}
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

func FindDeployment(snapshot Snapshot, app string) (Deployment, bool) {
	for _, deployment := range snapshot.Deployments {
		if deployment.App == app {
			return deployment, true
		}
	}
	return Deployment{}, false
}
