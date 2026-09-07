package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/minideploy"
)

type fakeDeploymentMetricsSource struct {
	snapshot minideploy.Snapshot
	err      error
}

func (f fakeDeploymentMetricsSource) Deployments(context.Context) (minideploy.Snapshot, error) {
	return f.snapshot, f.err
}

func TestAdminDeployments(t *testing.T) {
	source := fakeDeploymentMetricsSource{
		snapshot: minideploy.Snapshot{
			Deployments: []minideploy.Deployment{
				{
					App:      "portfolio",
					Strategy: "vite-static",
					Status:   "healthy",
				},
			},
			CollectedAt: time.Date(2026, 9, 7, 4, 2, 27, 0, time.UTC),
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil)
	w := httptest.NewRecorder()
	newHandler("", source).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); !containsAll(got, `"portfolio"`, `"vite-static"`, `"healthy"`) {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestAdminDeployment(t *testing.T) {
	source := fakeDeploymentMetricsSource{
		snapshot: minideploy.Snapshot{
			Deployments: []minideploy.Deployment{
				{
					App:      "golfmullet",
					Strategy: "fullstack-vite-node",
					Status:   "healthy",
				},
			},
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/golfmullet", nil)
	w := httptest.NewRecorder()
	newHandler("", source).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); !containsAll(got, `"golfmullet"`, `"fullstack-vite-node"`) {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestAdminDeploymentNotFound(t *testing.T) {
	source := fakeDeploymentMetricsSource{
		snapshot: minideploy.Snapshot{
			Deployments: []minideploy.Deployment{
				{App: "portfolio"},
			},
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/missing", nil)
	w := httptest.NewRecorder()
	newHandler("", source).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAdminDeploymentsUnavailable(t *testing.T) {
	source := fakeDeploymentMetricsSource{
		err: errors.New("MiniDeploy unavailable"),
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil)
	w := httptest.NewRecorder()
	newHandler("", source).ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestGuestDeploymentsEndpointDoesNotExist(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/guest/deployments", nil)
	w := httptest.NewRecorder()
	newHandler("", fakeDeploymentMetricsSource{}).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
