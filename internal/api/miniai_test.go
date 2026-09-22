package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/intelligence"
	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/observability"
	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

var miniAITestNow = time.Date(2026, 9, 22, 18, 0, 0, 0, time.UTC)

type fakeMiniAIDeployments struct {
	deployments []minideploy.DeploymentMetadata
	history     minideploy.DeploymentHistory
	metadataErr error
	historyErr  error
}

func (f fakeMiniAIDeployments) DeploymentMetadata(
	context.Context,
) ([]minideploy.DeploymentMetadata, error) {
	return f.deployments, f.metadataErr
}

func (f fakeMiniAIDeployments) DeploymentHistory(
	context.Context,
	string,
) (minideploy.DeploymentHistory, error) {
	return f.history, f.historyErr
}

type fakeMiniAIDatabases struct {
	snapshot    minibase.Snapshot
	backups     []minibase.Backup
	databaseErr error
	backupErr   error
}

func (f fakeMiniAIDatabases) ValidatedDatabases(
	context.Context,
) (minibase.Snapshot, error) {
	return f.snapshot, f.databaseErr
}

func (f fakeMiniAIDatabases) DatabaseBackups(
	context.Context,
	string,
) ([]minibase.Backup, error) {
	return f.backups, f.backupErr
}

type fakeMiniAIObservability struct {
	mu           sync.Mutex
	lastRange    observability.Range
	queryErr     error
	host         []observability.HostPoint
	temperatures []observability.TemperaturePoint
	applications []observability.ApplicationSummary
	application  []observability.ApplicationPoint
	services     []observability.ServiceSeries
	events       []observability.Event
	recoveries   []observability.RecoveryIncident
	recoveryErr  error
}

func (f *fakeMiniAIObservability) remember(window observability.Range) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastRange = window
}

func (f *fakeMiniAIObservability) QueryHost(
	_ context.Context,
	window observability.Range,
) ([]observability.HostPoint, error) {
	f.remember(window)
	return f.host, f.queryErr
}

func (f *fakeMiniAIObservability) QueryTemperature(
	_ context.Context,
	window observability.Range,
) ([]observability.TemperaturePoint, error) {
	f.remember(window)
	return f.temperatures, f.queryErr
}

func (f *fakeMiniAIObservability) QueryApplications(
	_ context.Context,
	window observability.Range,
) ([]observability.ApplicationSummary, error) {
	f.remember(window)
	return f.applications, f.queryErr
}

func (f *fakeMiniAIObservability) QueryApplication(
	_ context.Context,
	_ string,
	window observability.Range,
) ([]observability.ApplicationPoint, error) {
	f.remember(window)
	return f.application, f.queryErr
}

func (f *fakeMiniAIObservability) QueryServices(
	_ context.Context,
	window observability.Range,
) ([]observability.ServiceSeries, error) {
	f.remember(window)
	return f.services, f.queryErr
}

func (f *fakeMiniAIObservability) QueryEvents(
	_ context.Context,
	from time.Time,
	to time.Time,
	_ int,
) ([]observability.Event, error) {
	f.remember(observability.Range{From: from, To: to})
	return f.events, f.queryErr
}

func (f *fakeMiniAIObservability) LatestRecoveryIncident(
	context.Context,
) (*observability.RecoveryIncident, error) {
	if len(f.recoveries) == 0 {
		return nil, f.recoveryErr
	}
	latest := f.recoveries[0]
	return &latest, f.recoveryErr
}

func (f *fakeMiniAIObservability) ListRecoveryIncidents(
	context.Context,
	int,
) ([]observability.RecoveryIncident, error) {
	return f.recoveries, f.recoveryErr
}

func (f *fakeMiniAIObservability) RecoveryIncidentByID(
	context.Context,
	string,
) (*observability.RecoveryIncident, error) {
	return nil, nil
}

type boundedActivitySource struct {
	events    []history.ActivityEvent
	err       error
	lastLimit int
}

func (f *boundedActivitySource) ListActivity(
	_ context.Context,
	limit int,
) ([]history.ActivityEvent, error) {
	f.lastLimit = limit
	return f.events, f.err
}

func miniAITestSources() (
	fakeMiniAIDeployments,
	fakeMiniAIDatabases,
	*fakeMiniAIObservability,
	*boundedActivitySource,
) {
	commit := strings.Repeat("a", 40)
	previousCommit := strings.Repeat("b", 40)
	imageID := "sha256:" + strings.Repeat("c", 64)
	activatedAt := miniAITestNow.Add(-time.Hour)
	previousActivatedAt := activatedAt.Add(-24 * time.Hour)
	databaseID := "database_11111111111111111111111111111111"
	deployments := fakeMiniAIDeployments{
		deployments: []minideploy.DeploymentMetadata{{
			App:      "portfolio",
			Strategy: "fullstack-vite-node",
			Status:   "running",
			ImageID:  imageID,
			Source: &minideploy.Source{
				Provider:   "github",
				Repository: "owner/portfolio",
				Branch:     "main",
				CommitSHA:  commit,
			},
			ActivatedAt: &activatedAt,
			Services: []minideploy.ServiceMetadata{{
				Name:     "backend",
				Strategy: "node-express",
				Status:   "running",
				ImageID:  imageID,
			}},
			DatabaseAttachments: []minideploy.DatabaseAttachment{{
				DatabaseID:  databaseID,
				DisplayName: "Portfolio",
				BindingName: "primary",
			}},
		}, {
			App:                 "legacy",
			Strategy:            "vite-static",
			Status:              "running",
			Services:            []minideploy.ServiceMetadata{},
			DatabaseAttachments: []minideploy.DatabaseAttachment{},
		}},
		history: minideploy.DeploymentHistory{
			App: "portfolio",
			Versions: []minideploy.DeploymentVersion{{
				App:      "portfolio",
				Strategy: "fullstack-vite-node",
				Source: &minideploy.Source{
					Branch:    "main",
					CommitSHA: commit,
				},
				ActivatedAt: &activatedAt,
				ArchivedAt:  miniAITestNow.Add(-30 * time.Minute),
				ImageID:     imageID,
				Services: []minideploy.ServiceMetadata{{
					Name:    "backend",
					ImageID: imageID,
				}},
			}, {
				App:      "portfolio",
				Strategy: "fullstack-vite-node",
				Source: &minideploy.Source{
					Branch:    "release",
					CommitSHA: previousCommit,
				},
				ActivatedAt: &previousActivatedAt,
				ArchivedAt:  activatedAt,
				Services:    []minideploy.ServiceMetadata{},
			}},
		},
	}
	databases := fakeMiniAIDatabases{
		snapshot: minibase.Snapshot{
			Databases: []minibase.Database{{
				ID:          databaseID,
				DisplayName: "Portfolio",
				Status:      "ready",
				Attachments: []minibase.Attachment{{
					ConsumerType: "minideploy",
					ConsumerRef:  "portfolio",
					BindingName:  "primary",
				}},
				SizeBytes:         1024,
				Connections:       3,
				ActiveConnections: 1,
				IdleConnections:   2,
				Transactions:      minibase.Transactions{Commits: 10},
				Cache:             minibase.CacheMetrics{BlockHits: 20},
				Rows:              minibase.RowMetrics{Inserted: 5},
				BackupCount:       1,
				BackupBytes:       512,
			}},
			CollectedAt: miniAITestNow,
		},
		backups: []minibase.Backup{{
			ID:         "backup_22222222222222222222222222222222",
			DatabaseID: databaseID,
			Kind:       "automatic",
			Status:     "ready",
			SizeBytes:  512,
			CreatedAt:  miniAITestNow.Add(-time.Hour),
		}},
	}
	recovery := observability.RecoveryIncident{
		EventID:          strings.Repeat("d", 64),
		LastKnownAliveAt: miniAITestNow.Add(-10 * time.Minute),
		RecoveredAt:      miniAITestNow.Add(-5 * time.Minute),
		DowntimeSeconds:  300,
		Status:           "recovered",
		PreviousBootID:   "private-previous-boot",
		RecoveryBootID:   "private-recovery-boot",
	}
	observabilitySource := &fakeMiniAIObservability{
		host: []observability.HostPoint{{
			Timestamp:   miniAITestNow.Add(-time.Minute),
			SampleCount: 1,
		}},
		applications: []observability.ApplicationSummary{{
			ID:             "portfolio",
			Name:           "Portfolio",
			LatestStatus:   "healthy",
			LastObservedAt: miniAITestNow,
		}},
		application: []observability.ApplicationPoint{{
			Timestamp:   miniAITestNow.Add(-time.Minute),
			SampleCount: 1,
			Status:      "healthy",
		}},
		services: []observability.ServiceSeries{{
			ID:   "minideploy",
			Name: "MiniDeploy",
			Points: []observability.ServicePoint{{
				Timestamp:   miniAITestNow.Add(-time.Minute),
				SampleCount: 1,
				Available:   true,
				Status:      "healthy",
			}},
		}},
		events: []observability.Event{{
			ID:           strings.Repeat("e", 64),
			Source:       "reactorlab",
			Type:         "service_unavailable",
			ResourceType: "service",
			ResourceID:   "minibase",
			OccurredAt:   miniAITestNow.Add(-time.Minute),
			Summary:      "MiniBase was temporarily unavailable.",
			Details: map[string]any{
				"private": "MUST_NOT_PASS",
			},
		}},
		recoveries: []observability.RecoveryIncident{recovery},
	}
	activity := &boundedActivitySource{events: []history.ActivityEvent{{
		ID:          7,
		OccurredAt:  miniAITestNow.Add(-2 * time.Minute),
		Source:      "minibase",
		Kind:        "database_backup",
		Severity:    "info",
		Subject:     "Database backup",
		Message:     "Database backup completed.",
		Fingerprint: "private:fingerprint",
	}}}
	return deployments, databases, observabilitySource, activity
}

func newMiniAITestHandler(
	deployments miniAIDeploymentSource,
	databases miniAIDatabaseSource,
	observabilitySource observabilitySource,
	activity activitySource,
) *Handler {
	handler := newHandlerWithAllSources("", nil, nil, activity).(*Handler)
	handler.miniAIDeploy = deployments
	handler.miniAIBase = databases
	handler.observability = observabilitySource
	handler.now = func() time.Time { return miniAITestNow }
	handler.collectSystem = func() (reactorsystem.Metrics, error) {
		return reactorsystem.Metrics{
			CPU: reactorsystem.CPUStats{LogicalCores: 8},
			Services: []reactorsystem.ServiceStatus{{
				Name:   "MiniDeploy",
				Unit:   "minideploy.service",
				Status: "active",
				Active: true,
			}},
			CollectedAt: miniAITestNow,
		}, nil
	}
	handler.inspectHardwareWatchdogProtection = func(
		context.Context,
	) (reactorsystem.HardwareWatchdogProtectionState, error) {
		return reactorsystem.HardwareWatchdogProtectionState{
			State:          reactorsystem.RecoveryProtectionArmed,
			Identity:       "iTCO_wdt",
			TimeoutSeconds: 60,
		}, nil
	}
	handler.inspectRTCRecoveryProtection = func(
		context.Context,
	) (reactorsystem.RTCRecoveryProtectionState, error) {
		wakeAt := miniAITestNow.Add(5 * time.Minute)
		return reactorsystem.RTCRecoveryProtectionState{
			State:  reactorsystem.RecoveryProtectionArmed,
			WakeAt: &wakeAt,
		}, nil
	}
	return handler
}

func TestMiniAIRoutesExistOnlyOnPrivateHandlerAndAreGETOnly(t *testing.T) {
	deployments, databases, source, activity := miniAITestSources()
	private := newMiniAITestHandler(
		deployments,
		databases,
		source,
		activity,
	)
	routes := []string{
		"/internal/miniai/v1/overview",
		"/internal/miniai/v1/observability/host?range=1h",
		"/internal/miniai/v1/observability/temperature?range=1h",
		"/internal/miniai/v1/observability/applications?range=1h",
		"/internal/miniai/v1/observability/applications/portfolio?range=1h",
		"/internal/miniai/v1/observability/services?range=1h",
		"/internal/miniai/v1/observability/events?range=1h",
		"/internal/miniai/v1/deployments",
		"/internal/miniai/v1/deployments/portfolio",
		"/internal/miniai/v1/deployments/portfolio/history",
		"/internal/miniai/v1/databases",
		"/internal/miniai/v1/databases/database_11111111111111111111111111111111",
		"/internal/miniai/v1/databases/database_11111111111111111111111111111111/backups",
		"/internal/miniai/v1/activity",
		"/internal/miniai/v1/recovery",
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			response := httptest.NewRecorder()
			private.ServeHTTP(
				response,
				httptest.NewRequest(http.MethodGet, route, nil),
			)
			if response.Code != http.StatusOK {
				t.Fatalf(
					"private GET status = %d, body=%s",
					response.Code,
					response.Body.String(),
				)
			}

			publicResponse := httptest.NewRecorder()
			newPublicHandler("", nil, nil).ServeHTTP(
				publicResponse,
				httptest.NewRequest(http.MethodGet, route, nil),
			)
			if publicResponse.Code != http.StatusNotFound {
				t.Fatalf(
					"public GET status = %d, want 404",
					publicResponse.Code,
				)
			}

			writeResponse := httptest.NewRecorder()
			private.ServeHTTP(
				writeResponse,
				httptest.NewRequest(http.MethodPost, route, nil),
			)
			if writeResponse.Code != http.StatusMethodNotAllowed {
				t.Fatalf(
					"private POST status = %d, want 405",
					writeResponse.Code,
				)
			}
		})
	}
}

func TestMiniAIOverviewSupportsIndependentPartialAvailability(t *testing.T) {
	tests := []struct {
		name             string
		deploymentErr    error
		databaseErr      error
		observabilityErr error
		unavailableKey   string
		errorCode        string
	}{
		{
			name:           "MiniBase unavailable",
			databaseErr:    errors.New("private database password"),
			unavailableKey: "databases",
			errorCode:      "database_source_unavailable",
		},
		{
			name:           "MiniDeploy unavailable",
			deploymentErr:  errors.New("private deployment token"),
			unavailableKey: "deployments",
			errorCode:      "deployment_source_unavailable",
		},
		{
			name:             "observability unavailable",
			observabilityErr: errors.New("private SQLite path"),
			unavailableKey:   "observability",
			errorCode:        "observability_unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deployments, databases, source, activity := miniAITestSources()
			deployments.metadataErr = test.deploymentErr
			databases.databaseErr = test.databaseErr
			source.queryErr = test.observabilityErr
			handler := newMiniAITestHandler(
				deployments,
				databases,
				source,
				activity,
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(
				response,
				httptest.NewRequest(
					http.MethodGet,
					"/internal/miniai/v1/overview",
					nil,
				),
			)
			if response.Code != http.StatusOK {
				t.Fatalf(
					"status = %d, body=%s",
					response.Code,
					response.Body.String(),
				)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"system", "recovery"} {
				section := body[key].(map[string]any)
				if section["available"] != true {
					t.Fatalf("%s section = %#v", key, section)
				}
			}
			section := body[test.unavailableKey].(map[string]any)
			if section["available"] != false ||
				section["error"] != test.errorCode {

				t.Fatalf(
					"%s section = %#v",
					test.unavailableKey,
					section,
				)
			}
			for _, secret := range []string{
				"private database password",
				"private deployment token",
				"private SQLite path",
			} {
				if strings.Contains(response.Body.String(), secret) {
					t.Fatalf(
						"overview leaked upstream error %q: %s",
						secret,
						response.Body.String(),
					)
				}
			}
		})
	}
}

func TestMiniAIOverviewAllSourcesAvailable(t *testing.T) {
	deployments, databases, source, activity := miniAITestSources()
	response := httptest.NewRecorder()
	newMiniAITestHandler(
		deployments,
		databases,
		source,
		activity,
	).ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/overview",
			nil,
		),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var overview intelligence.Overview
	if err := json.Unmarshal(response.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if !overview.System.Available || !overview.Recovery.Available ||
		!overview.Deployments.Available || !overview.Databases.Available ||
		!overview.Observability.Available {

		t.Fatalf("overview = %#v", overview)
	}
}

func TestMiniAIExplicitWindowAndPointBound(t *testing.T) {
	deployments, databases, source, activity := miniAITestSources()
	handler := newMiniAITestHandler(
		deployments,
		databases,
		source,
		activity,
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/observability/host?from=2026-09-22T10:00:00-04:00&to=2026-09-22T10:20:00-04:00",
			nil,
		),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	source.mu.Lock()
	gotWindow := source.lastRange
	source.mu.Unlock()
	if !gotWindow.From.Equal(
		time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC),
	) || gotWindow.MaxPoints != intelligence.MaxMetricPoints {

		t.Fatalf("resolved window = %#v", gotWindow)
	}

	for _, query := range []string{
		"?from=2026-09-22T17:00:00Z",
		"?from=2026-09-22T17:00:00Z&to=2026-09-22T17:00:00Z",
		"?from=2026-09-15T17:59:59Z&to=2026-09-22T18:00:00Z",
	} {
		invalid := httptest.NewRecorder()
		handler.ServeHTTP(
			invalid,
			httptest.NewRequest(
				http.MethodGet,
				"/internal/miniai/v1/observability/host"+query,
				nil,
			),
		)
		if invalid.Code != http.StatusBadRequest ||
			!strings.Contains(invalid.Body.String(), "invalid_window") {

			t.Fatalf(
				"query %q status=%d body=%s",
				query,
				invalid.Code,
				invalid.Body.String(),
			)
		}
	}

	source.host = make(
		[]observability.HostPoint,
		intelligence.MaxMetricPoints+1,
	)
	oversized := httptest.NewRecorder()
	handler.ServeHTTP(
		oversized,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/observability/host?range=1h",
			nil,
		),
	)
	if oversized.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"oversized status = %d, body=%s",
			oversized.Code,
			oversized.Body.String(),
		)
	}
}

func TestMiniAIDeploymentProjectionPreservesIdentityWithoutRawRepoURL(t *testing.T) {
	deployments, databases, source, activity := miniAITestSources()
	handler := newMiniAITestHandler(
		deployments,
		databases,
		source,
		activity,
	)
	for _, route := range []string{
		"/internal/miniai/v1/deployments",
		"/internal/miniai/v1/deployments/portfolio",
		"/internal/miniai/v1/deployments/portfolio/history",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			httptest.NewRequest(http.MethodGet, route, nil),
		)
		if response.Code != http.StatusOK {
			t.Fatalf(
				"%s status=%d body=%s",
				route,
				response.Code,
				response.Body.String(),
			)
		}
		body := response.Body.String()
		for _, expected := range []string{
			strings.Repeat("a", 40),
			"main",
			"activatedAt",
			"imageId",
		} {
			if !strings.Contains(body, expected) {
				t.Fatalf("%s lacks %q: %s", route, expected, body)
			}
		}
		for _, forbidden := range []string{
			"repoUrl",
			"environment",
			"DATABASE_URL",
			"private-previous-boot",
		} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s leaked %q: %s", route, forbidden, body)
			}
		}
		if strings.HasSuffix(route, "/history") {
			if !strings.Contains(body, strings.Repeat("b", 40)) ||
				!strings.Contains(body, "archivedAt") {

				t.Fatalf("history identity/times missing: %s", body)
			}
		}
	}

	legacy := httptest.NewRecorder()
	handler.ServeHTTP(
		legacy,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/deployments/legacy",
			nil,
		),
	)
	if legacy.Code != http.StatusOK {
		t.Fatalf("legacy status=%d body=%s", legacy.Code, legacy.Body.String())
	}
	if strings.Contains(legacy.Body.String(), "source") ||
		strings.Contains(legacy.Body.String(), "activatedAt") {

		t.Fatalf("legacy provenance was invented: %s", legacy.Body.String())
	}
}

func TestMiniAIDatabaseAndBackupProjectionIsSafeAndBounded(t *testing.T) {
	deployments, databases, source, activity := miniAITestSources()
	databaseID := "database_11111111111111111111111111111111"
	databases.backups = make([]minibase.Backup, 0, 3)
	for index := 0; index < 3; index++ {
		databases.backups = append(databases.backups, minibase.Backup{
			ID:         fmt.Sprintf("backup_%032x", index+1),
			DatabaseID: databaseID,
			Kind:       "automatic",
			Status:     "ready",
			SizeBytes:  int64(index + 1),
			CreatedAt: miniAITestNow.Add(
				-time.Duration(index) * time.Hour,
			),
		})
	}
	handler := newMiniAITestHandler(
		deployments,
		databases,
		source,
		activity,
	)
	databaseResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		databaseResponse,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/databases/"+databaseID,
			nil,
		),
	)
	if databaseResponse.Code != http.StatusOK ||
		!strings.Contains(databaseResponse.Body.String(), "\"backupCount\":1") ||
		!strings.Contains(databaseResponse.Body.String(), "\"commits\":10") {

		t.Fatalf(
			"database status=%d body=%s",
			databaseResponse.Code,
			databaseResponse.Body.String(),
		)
	}

	backupResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		backupResponse,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/databases/"+databaseID+"/backups?limit=2",
			nil,
		),
	)
	if backupResponse.Code != http.StatusOK {
		t.Fatalf(
			"backup status=%d body=%s",
			backupResponse.Code,
			backupResponse.Body.String(),
		)
	}
	var backups intelligence.BackupCollection
	if err := json.Unmarshal(backupResponse.Body.Bytes(), &backups); err != nil {
		t.Fatal(err)
	}
	if len(backups.Backups) != 2 || !backups.Truncated ||
		backups.Limit != 2 {

		t.Fatalf("backups = %#v", backups)
	}
	for _, forbidden := range []string{
		"path", "password", "credential", "connectionString",
	} {
		if strings.Contains(backupResponse.Body.String(), forbidden) {
			t.Fatalf(
				"backup response leaked %q: %s",
				forbidden,
				backupResponse.Body.String(),
			)
		}
	}
}

func TestMiniAIRecoveryAndActivityOmitPrivateIdentityAndRemainBounded(t *testing.T) {
	deployments, databases, source, activity := miniAITestSources()
	activity.events = make([]history.ActivityEvent, 0, 205)
	for index := 0; index < 205; index++ {
		activity.events = append(activity.events, history.ActivityEvent{
			ID: int64(index),
			OccurredAt: miniAITestNow.Add(
				-time.Duration(index) * time.Minute,
			),
			Source:      "reactorlab",
			Kind:        "platform",
			Severity:    "info",
			Subject:     fmt.Sprintf("event-%03d", index),
			Message:     "Safe activity.",
			Fingerprint: "private-fingerprint",
		})
	}
	handler := newMiniAITestHandler(
		deployments,
		databases,
		source,
		activity,
	)

	recoveryResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		recoveryResponse,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/recovery",
			nil,
		),
	)
	if recoveryResponse.Code != http.StatusOK ||
		!strings.Contains(recoveryResponse.Body.String(), "\"state\":\"armed\"") {

		t.Fatalf(
			"recovery status=%d body=%s",
			recoveryResponse.Code,
			recoveryResponse.Body.String(),
		)
	}
	for _, forbidden := range []string{
		"previousBootId",
		"recoveryBootId",
		"private-previous-boot",
		"private-recovery-boot",
	} {
		if strings.Contains(recoveryResponse.Body.String(), forbidden) {
			t.Fatalf(
				"recovery leaked %q: %s",
				forbidden,
				recoveryResponse.Body.String(),
			)
		}
	}

	activityResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		activityResponse,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/activity?limit=200",
			nil,
		),
	)
	if activityResponse.Code != http.StatusOK {
		t.Fatalf(
			"activity status=%d body=%s",
			activityResponse.Code,
			activityResponse.Body.String(),
		)
	}
	var result intelligence.ActivityResponse
	if err := json.Unmarshal(activityResponse.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != intelligence.MaxActivityResults ||
		result.Events[0].OccurredAt.Before(
			result.Events[len(result.Events)-1].OccurredAt,
		) || activity.lastLimit != intelligence.MaxActivityResults {

		t.Fatalf("activity = %#v, requested limit=%d", result, activity.lastLimit)
	}
	if strings.Contains(activityResponse.Body.String(), "fingerprint") {
		t.Fatalf(
			"activity leaked internal fingerprint: %s",
			activityResponse.Body.String(),
		)
	}
}

func TestMiniAIObservabilityEventsDiscardUntypedDetails(t *testing.T) {
	deployments, databases, source, activity := miniAITestSources()
	response := httptest.NewRecorder()
	newMiniAITestHandler(
		deployments,
		databases,
		source,
		activity,
	).ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/internal/miniai/v1/observability/events?range=1h",
			nil,
		),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "MUST_NOT_PASS") ||
		strings.Contains(response.Body.String(), "details") {

		t.Fatalf("event response leaked raw details: %s", response.Body.String())
	}
}
