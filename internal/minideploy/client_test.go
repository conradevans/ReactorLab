package minideploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientDeployments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/reactorlab/deployments" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"deployments": [{
				"app": "portfolio",
				"strategy": "vite-static",
				"status": "healthy",
				"containers": [{
					"service": "app",
					"strategy": "vite-static",
					"container": "minideploy-portfolio",
					"state": "running",
					"health": "healthy",
					"cpuPercent": 0.2,
					"memoryUsedBytes": 1024,
					"memoryLimitBytes": 4096,
					"memoryPercent": 25,
					"networkRxBytes": 10,
					"networkTxBytes": 20,
					"blockReadBytes": 30,
					"blockWriteBytes": 40,
					"pids": 4,
					"writableBytes": 50,
					"uptimeSeconds": 60,
					"restartCount": 1
				}]
			}],
			"collectedAt": "2026-09-07T04:02:27Z"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	snapshot, err := client.Deployments(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Deployments) != 1 {
		t.Fatalf("deployments = %d, want 1", len(snapshot.Deployments))
	}
	deployment := snapshot.Deployments[0]
	if deployment.App != "portfolio" || deployment.Status != "healthy" {
		t.Fatalf("unexpected deployment: %#v", deployment)
	}
	if len(deployment.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(deployment.Containers))
	}
	if deployment.Containers[0].MemoryUsedBytes != 1024 {
		t.Fatalf("memory used = %d", deployment.Containers[0].MemoryUsedBytes)
	}
}

func TestClientRejectsNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	if _, err := client.Deployments(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindDeployment(t *testing.T) {
	snapshot := Snapshot{
		Deployments: []Deployment{
			{App: "golfmullet"},
			{App: "myscheduler"},
		},
	}

	deployment, ok := FindDeployment(snapshot, "myscheduler")
	if !ok {
		t.Fatal("deployment not found")
	}
	if deployment.App != "myscheduler" {
		t.Fatalf("app = %q", deployment.App)
	}

	if _, ok := FindDeployment(snapshot, "missing"); ok {
		t.Fatal("unexpected deployment match")
	}
}

func TestClientGuestDeploymentsUsesGuestContractAndDiscardsUnknownFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/guest/deployments" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"summary":{"total":3,"showing":1,"hidden":2},
			"deployments":[{
				"app":"portfolio",
				"url":"https://portfolio.reactorlab.dev",
				"status":"running",
				"container":"must-not-pass",
				"port":8080
			}],
			"internal":"must-not-pass"
		}`))
	}))
	defer server.Close()

	result, err := NewClient(server.URL, time.Second).GuestDeployments(context.Background())
	if err != nil {
		t.Fatalf("GuestDeployments() error = %v", err)
	}
	if result.Summary != (GuestSummary{Total: 3, Showing: 1, Hidden: 2}) {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if len(result.Deployments) != 1 ||
		result.Deployments[0] != (GuestDeployment{
			App: "portfolio", URL: "https://portfolio.reactorlab.dev", Status: "running",
		}) {
		t.Fatalf("deployments = %#v", result.Deployments)
	}

	encoded, err := json.Marshal(result.Deployments[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"app":"portfolio","url":"https://portfolio.reactorlab.dev","status":"running"}` {
		t.Fatalf("Guest deployment fields = %s", encoded)
	}
}

func TestClientGuestDeploymentsRejectsMalformedOrInvalidResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{`},
		{name: "missing summary", body: `{"deployments":[]}`},
		{name: "missing deployments", body: `{"summary":{"total":0,"showing":0,"hidden":0}}`},
		{name: "negative count", body: `{"summary":{"total":-1,"showing":0,"hidden":-1},"deployments":[]}`},
		{name: "showing mismatch", body: `{"summary":{"total":1,"showing":1,"hidden":0},"deployments":[]}`},
		{name: "hidden invariant", body: `{"summary":{"total":3,"showing":1,"hidden":1},"deployments":[{"app":"a","url":"https://a.example","status":"running"}]}`},
		{name: "empty item field", body: `{"summary":{"total":1,"showing":1,"hidden":0},"deployments":[{"app":"","url":"https://a.example","status":"running"}]}`},
		{name: "invalid URL", body: `{"summary":{"total":1,"showing":1,"hidden":0},"deployments":[{"app":"a","url":"localhost:8080","status":"running"}]}`},
		{name: "trailing JSON", body: `{"summary":{"total":0,"showing":0,"hidden":0},"deployments":[]} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			if _, err := NewClient(server.URL, time.Second).GuestDeployments(context.Background()); err == nil {
				t.Fatal("GuestDeployments() error = nil")
			}
		})
	}
}

func TestValidateGuestDeploymentURL(t *testing.T) {
	valid := GuestDeployment{
		App: "portfolio", URL: "https://portfolio.reactorlab.dev",
		Status: "running",
	}
	if err := validateGuestDeployment(valid); err != nil {
		t.Fatalf("validateGuestDeployment() error = %v", err)
	}

	tests := []struct {
		name string
		url  string
	}{
		{name: "HTTP", url: "http://portfolio.reactorlab.dev"},
		{name: "JavaScript", url: "javascript:alert(1)"},
		{name: "data", url: "data:text/html,unsafe"},
		{name: "file", url: "file:///etc/passwd"},
		{name: "missing scheme", url: "portfolio.reactorlab.dev"},
		{name: "malformed", url: "https://[::1"},
		{name: "trailing whitespace", url: "https://example.com trailing"},
		{name: "localhost", url: "https://localhost"},
		{name: "localhost with port", url: "https://localhost:3000"},
		{name: "localhost subdomain", url: "https://app.localhost"},
		{name: "local-only hostname", url: "https://app.local"},
		{name: "IPv4 loopback", url: "https://127.0.0.1"},
		{name: "other IPv4 loopback", url: "https://127.0.0.2"},
		{name: "IPv6 loopback", url: "https://[::1]"},
		{name: "mapped IPv4 loopback", url: "https://[::ffff:127.0.0.1]"},
		{name: "IPv4 unspecified", url: "https://0.0.0.0"},
		{name: "IPv6 unspecified", url: "https://[::]"},
		{name: "private address", url: "https://10.0.0.1"},
		{name: "link-local address", url: "https://169.254.1.1"},
		{name: "zoned IPv6 link-local", url: "https://[fe80::1%25eth0]"},
		{name: "username", url: "https://user@example.com"},
		{name: "username and password", url: "https://user:password@example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deployment := valid
			deployment.URL = test.url
			if err := validateGuestDeployment(deployment); err == nil {
				t.Fatalf("validateGuestDeployment(%q) error = nil", test.url)
			}
		})
	}
}

func TestClientGuestDeploymentsRejectsNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private upstream detail", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, time.Second).GuestDeployments(context.Background()); err == nil {
		t.Fatal("GuestDeployments() error = nil")
	}
}

func TestClientGuestDeploymentsTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"summary":{"total":0,"showing":0,"hidden":0},"deployments":[]}`))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, 5*time.Millisecond).GuestDeployments(context.Background()); err == nil {
		t.Fatal("GuestDeployments() error = nil")
	}
}
