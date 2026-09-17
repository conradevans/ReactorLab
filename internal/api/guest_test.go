package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
)

type fakeGuestDeploymentSource struct {
	snapshot minideploy.GuestDeployments
	err      error
}

func (source fakeGuestDeploymentSource) GuestDeployments(
	context.Context,
) (minideploy.GuestDeployments, error) {
	return source.snapshot, source.err
}

type fakeGuestDatabaseSource struct {
	snapshot minibase.GuestDatabases
	err      error
}

func (source fakeGuestDatabaseSource) GuestDatabases(
	context.Context,
) (minibase.GuestDatabases, error) {
	return source.snapshot, source.err
}

func availableGuestDeploymentSource() fakeGuestDeploymentSource {
	return fakeGuestDeploymentSource{
		snapshot: minideploy.GuestDeployments{
			Summary: minideploy.GuestSummary{
				Total: 3, Showing: 1, Hidden: 2,
			},
			Deployments: []minideploy.GuestDeployment{
				{
					App: "portfolio", URL: "https://portfolio.reactorlab.dev",
					Status: "running",
				},
			},
		},
	}
}

func availableGuestDatabaseSource() fakeGuestDatabaseSource {
	return fakeGuestDatabaseSource{
		snapshot: minibase.GuestDatabases{
			Summary: minibase.GuestSummary{
				Total: 2, Showing: 1, Hidden: 1,
			},
			Databases: []minibase.GuestDatabase{
				{
					ID: "database_a", DisplayName: "Shared Database",
					Status: "ready",
				},
			},
		},
	}
}

func emptyGuestDeploymentSource() fakeGuestDeploymentSource {
	return fakeGuestDeploymentSource{
		snapshot: minideploy.GuestDeployments{
			Summary:     minideploy.GuestSummary{},
			Deployments: []minideploy.GuestDeployment{},
		},
	}
}

func emptyGuestDatabaseSource() fakeGuestDatabaseSource {
	return fakeGuestDatabaseSource{
		snapshot: minibase.GuestDatabases{
			Summary:   minibase.GuestSummary{},
			Databases: []minibase.GuestDatabase{},
		},
	}
}

func requestGuestResources(
	t *testing.T,
	deployments guestDeploymentSource,
	databases guestDatabaseSource,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/guest/resources",
		nil,
	)
	response := httptest.NewRecorder()
	newHandlerWithGuestSources("", deployments, databases).
		ServeHTTP(response, request)
	return response
}

func TestGuestResourcesMapsCountsAndUsesExplicitDTOs(t *testing.T) {
	response := requestGuestResources(
		t,
		availableGuestDeploymentSource(),
		availableGuestDatabaseSource(),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	var payload guestResourcesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Deployments.Available ||
		payload.Deployments.Summary != (guestResourceSummary{
			Total: 3, Shared: 1, Hidden: 2,
		}) {
		t.Fatalf("deployments = %#v", payload.Deployments)
	}
	if !payload.Databases.Available ||
		payload.Databases.Summary != (guestResourceSummary{
			Total: 2, Shared: 1, Hidden: 1,
		}) {
		t.Fatalf("databases = %#v", payload.Databases)
	}
	if len(payload.Deployments.Items) != 1 ||
		payload.Deployments.Items[0] != (guestDeploymentItem{
			App: "portfolio", URL: "https://portfolio.reactorlab.dev",
			Status: "running",
		}) {
		t.Fatalf("deployment items = %#v", payload.Deployments.Items)
	}
	if len(payload.Databases.Items) != 1 ||
		payload.Databases.Items[0] != (guestDatabaseItem{
			ID: "database_a", DisplayName: "Shared Database", Status: "ready",
		}) {
		t.Fatalf("database items = %#v", payload.Databases.Items)
	}

	var raw map[string]struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if got := raw["deployments"].Items[0]; len(got) != 3 ||
		got["app"] != "portfolio" ||
		got["url"] != "https://portfolio.reactorlab.dev" ||
		got["status"] != "running" {
		t.Fatalf("deployment DTO = %#v", got)
	}
	if got := raw["databases"].Items[0]; len(got) != 3 ||
		got["id"] != "database_a" ||
		got["displayName"] != "Shared Database" ||
		got["status"] != "ready" {
		t.Fatalf("database DTO = %#v", got)
	}
}

func TestGuestResourcesUsesDistinctMiniDeployGuestListener(t *testing.T) {
	if minideploy.DefaultBaseURL != "http://127.0.0.1:9000" {
		t.Fatalf("private MiniDeploy base = %q", minideploy.DefaultBaseURL)
	}
	if minideploy.DefaultGuestBaseURL != "http://127.0.0.1:9003" {
		t.Fatalf("Guest MiniDeploy base = %q", minideploy.DefaultGuestBaseURL)
	}
	if minideploy.DefaultBaseURL == minideploy.DefaultGuestBaseURL {
		t.Fatal("private and Guest MiniDeploy bases must differ")
	}

	var privateRequests atomic.Int32
	privateServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			privateRequests.Add(1)
			if r.URL.Path == "/internal/reactorlab/deployments" {
				_, _ = w.Write([]byte(`{"deployments":[],"collectedAt":"2026-09-17T00:00:00Z"}`))
				return
			}
			http.NotFound(w, r)
		},
	))
	defer privateServer.Close()

	var guestRequests atomic.Int32
	guestServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			guestRequests.Add(1)
			if r.Method != http.MethodGet ||
				r.URL.Path != "/api/guest/deployments" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{
				"summary":{"total":1,"showing":1,"hidden":0},
				"deployments":[{
					"app":"portfolio",
					"url":"https://portfolio.reactorlab.dev",
					"status":"running"
				}]
			}`))
		},
	))
	defer guestServer.Close()

	miniBaseServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet ||
				r.URL.Path != "/api/v1/guest/databases" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{
				"summary":{"total":0,"showing":0,"hidden":0},
				"databases":[]
			}`))
		},
	))
	defer miniBaseServer.Close()

	clients := newServiceClients(
		privateServer.URL,
		guestServer.URL,
		miniBaseServer.URL,
	)
	handler := newHandlerWithServiceClients("", nil, nil, clients)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/guest/resources",
		nil,
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	var payload guestResourcesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Deployments.Available ||
		len(payload.Deployments.Items) != 1 ||
		payload.Deployments.Items[0].App != "portfolio" {
		t.Fatalf("deployments = %#v", payload.Deployments)
	}
	if got := privateRequests.Load(); got != 0 {
		t.Fatalf("private MiniDeploy requests = %d, want 0", got)
	}
	if got := guestRequests.Load(); got != 1 {
		t.Fatalf("Guest MiniDeploy requests = %d, want 1", got)
	}
}

func TestGuestResourcesIsolatesUpstreamFailures(t *testing.T) {
	privateError := errors.New(
		"dial http://127.0.0.1:9000: private upstream body and stack trace",
	)
	tests := []struct {
		name                 string
		deployments          guestDeploymentSource
		databases            guestDatabaseSource
		deploymentsAvailable bool
		databasesAvailable   bool
	}{
		{
			name:               "MiniDeploy unavailable",
			deployments:        fakeGuestDeploymentSource{err: privateError},
			databases:          availableGuestDatabaseSource(),
			databasesAvailable: true,
		},
		{
			name:                 "MiniBase unavailable",
			deployments:          availableGuestDeploymentSource(),
			databases:            fakeGuestDatabaseSource{err: privateError},
			deploymentsAvailable: true,
		},
		{
			name:        "both unavailable",
			deployments: fakeGuestDeploymentSource{err: privateError},
			databases:   fakeGuestDatabaseSource{err: privateError},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestGuestResources(
				t,
				test.deployments,
				test.databases,
			)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			var payload guestResourcesResponse
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Deployments.Available != test.deploymentsAvailable ||
				payload.Databases.Available != test.databasesAvailable {
				t.Fatalf("payload = %#v", payload)
			}
			if strings.Contains(response.Body.String(), "127.0.0.1") ||
				strings.Contains(response.Body.String(), "stack trace") ||
				strings.Contains(response.Body.String(), "private upstream") {
				t.Fatalf("raw internal error leaked: %s", response.Body.String())
			}
		})
	}
}

func TestGuestResourcesRejectsInvalidSourceInvariants(t *testing.T) {
	response := requestGuestResources(
		t,
		fakeGuestDeploymentSource{
			snapshot: minideploy.GuestDeployments{
				Summary: minideploy.GuestSummary{
					Total: 2, Showing: 1, Hidden: 0,
				},
				Deployments: []minideploy.GuestDeployment{
					{App: "a", URL: "https://a.example", Status: "running"},
				},
			},
		},
		fakeGuestDatabaseSource{
			snapshot: minibase.GuestDatabases{
				Summary: minibase.GuestSummary{
					Total: 1, Showing: 1, Hidden: 0,
				},
				Databases: []minibase.GuestDatabase{},
			},
		},
	)

	var payload guestResourcesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Deployments.Available || payload.Databases.Available {
		t.Fatalf("invalid snapshots were exposed: %#v", payload)
	}
	if payload.Deployments.Items == nil || payload.Databases.Items == nil {
		t.Fatalf("unavailable item lists must serialize as arrays: %#v", payload)
	}
}
