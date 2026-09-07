package minideploy

import (
	"context"
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
