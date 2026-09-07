package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
)

func joinedSources() (
	fakeDeploymentMetricsSource,
	fakeDatabaseMetricsSource,
) {
	deployments := fakeDeploymentMetricsSource{
		snapshot: minideploy.Snapshot{
			Deployments: []minideploy.Deployment{
				{
					App:      "myscheduler",
					Strategy: "fullstack-vite-node",
					Status:   "healthy",
				},
				{
					App:      "portfolio",
					Strategy: "vite-static",
					Status:   "healthy",
				},
			},
		},
	}

	databases := fakeDatabaseMetricsSource{
		snapshot: minibase.Snapshot{
			Databases: []minibase.Database{
				{
					ID:          "database_1",
					DisplayName: "MyScheduler Production",
					Status:      "ready",
					Attachments: []minibase.Attachment{
						{
							ConsumerType: "minideploy",
							ConsumerRef:  "myscheduler",
							BindingName:  "primary",
						},
					},
				},
				{
					ID:          "database_2",
					DisplayName: "Orphaned Database",
					Status:      "ready",
					Attachments: []minibase.Attachment{
						{
							ConsumerType: "minideploy",
							ConsumerRef:  "missing-app",
							BindingName:  "primary",
						},
					},
				},
			},
			Postgres: minibase.PostgresMetrics{State: "running"},
		},
	}

	return deployments, databases
}

func TestDeploymentListIncludesJoinedDatabase(t *testing.T) {
	deployments, databases := joinedSources()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", deployments, databases).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !containsAll(
		body,
		`"app":"myscheduler"`,
		`"database":{"state":"linked"`,
		`"id":"database_1"`,
		`"displayName":"MyScheduler Production"`,
		`"status":"ready"`,
	) {
		t.Fatalf("unexpected body: %s", body)
	}
	if strings.Contains(body, `"attachments"`) ||
		strings.Contains(body, `"consumerRef"`) {
		t.Fatalf("raw attachment metadata leaked: %s", body)
	}
}

func TestDeploymentResponseMarksDetachedDatabase(t *testing.T) {
	deployments, databases := joinedSources()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/portfolio", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", deployments, databases).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), `"database":{"state":"detached"}`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDeploymentResponseSurvivesMiniBaseFailure(t *testing.T) {
	deployments, _ := joinedSources()
	databases := fakeDatabaseMetricsSource{
		err: errors.New("MiniBase unavailable"),
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/myscheduler", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", deployments, databases).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), `"database":{"state":"unavailable"}`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDatabaseResponseIncludesJoinedDeployment(t *testing.T) {
	deployments, databases := joinedSources()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/databases/database_1", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", deployments, databases).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !containsAll(
		body,
		`"deployment":{"state":"linked"`,
		`"app":"myscheduler"`,
		`"strategy":"fullstack-vite-node"`,
		`"status":"healthy"`,
	) {
		t.Fatalf("unexpected body: %s", body)
	}
	if strings.Contains(body, `"attachments"`) ||
		strings.Contains(body, `"consumerRef"`) {
		t.Fatalf("raw attachment metadata leaked: %s", body)
	}
}

func TestDatabaseResponseMarksUnresolvedDeployment(t *testing.T) {
	deployments, databases := joinedSources()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/databases/database_2", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", deployments, databases).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !containsAll(
		w.Body.String(),
		`"deployment":{"state":"unresolved"`,
		`"app":"missing-app"`,
	) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}
