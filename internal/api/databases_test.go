package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/minibase"
)

type fakeDatabaseMetricsSource struct {
	snapshot minibase.Snapshot
	err      error
}

func (f fakeDatabaseMetricsSource) Databases(context.Context) (minibase.Snapshot, error) {
	return f.snapshot, f.err
}

func TestAdminDatabases(t *testing.T) {
	source := fakeDatabaseMetricsSource{
		snapshot: minibase.Snapshot{
			Databases: []minibase.Database{
				{
					ID:          "database_1",
					DisplayName: "MyScheduler Production",
					Status:      "ready",
					SizeBytes:   9000627,
					BackupCount: 5,
				},
			},
			Postgres: minibase.PostgresMetrics{
				State: "running",
			},
			CollectedAt: time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC),
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/databases", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", nil, source).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); !containsAll(
		got,
		`"MyScheduler Production"`,
		`"ready"`,
		`"postgres"`,
		`"running"`,
	) {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestAdminDatabase(t *testing.T) {
	source := fakeDatabaseMetricsSource{
		snapshot: minibase.Snapshot{
			Databases: []minibase.Database{
				{
					ID:          "database_1",
					DisplayName: "Golfmullet Production",
					Status:      "ready",
					BackupCount: 1,
				},
			},
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/databases/database_1", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", nil, source).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); !containsAll(got, `"database_1"`, `"Golfmullet Production"`) {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestAdminDatabaseNotFound(t *testing.T) {
	source := fakeDatabaseMetricsSource{
		snapshot: minibase.Snapshot{
			Databases: []minibase.Database{
				{ID: "database_1"},
			},
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/databases/missing", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", nil, source).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAdminDatabasesUnavailable(t *testing.T) {
	source := fakeDatabaseMetricsSource{
		err: errors.New("MiniBase unavailable"),
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/databases", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", nil, source).ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestGuestDatabasesEndpointDoesNotExist(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/guest/databases", nil)
	w := httptest.NewRecorder()
	newHandlerWithSources("", nil, fakeDatabaseMetricsSource{}).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
