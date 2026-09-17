package minibase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientDatabases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/reactorlab/databases" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"databases":[{
				"id":"database_1",
				"displayName":"MyScheduler Production",
				"status":"ready",
				"attachments":[{
					"consumerType":"minideploy",
					"consumerRef":"myscheduler",
					"bindingName":"primary"
				}],
				"sizeBytes":9000627,
				"connections":1,
				"activeConnections":0,
				"idleConnections":1,
				"transactions":{"commits":15305,"rollbacks":5},
				"cache":{"blockReads":204,"blockHits":735365},
				"rows":{"inserted":2570,"updated":487,"deleted":1267},
				"backupCount":5,
				"backupBytes":311048
			}],
			"postgres":{
				"state":"running",
				"cpuPercent":0,
				"memoryUsedBytes":76241961,
				"memoryLimitBytes":16073915105,
				"memoryPercent":0.47,
				"networkRxBytes":28800,
				"networkTxBytes":19200,
				"blockReadBytes":58400000,
				"blockWriteBytes":897000,
				"pids":7
			},
			"collectedAt":"2026-09-07T08:00:00Z"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	snapshot, err := client.Databases(context.Background())
	if err != nil {
		t.Fatalf("Databases() error = %v", err)
	}
	if len(snapshot.Databases) != 1 {
		t.Fatalf("database count = %d", len(snapshot.Databases))
	}
	database := snapshot.Databases[0]
	if database.DisplayName != "MyScheduler Production" ||
		database.Connections != 1 ||
		database.Transactions.Commits != 15305 ||
		database.BackupCount != 5 {
		t.Fatalf("database = %#v", database)
	}
	if len(database.Attachments) != 1 ||
		database.Attachments[0].ConsumerType != "minideploy" ||
		database.Attachments[0].ConsumerRef != "myscheduler" ||
		database.Attachments[0].BindingName != "primary" {
		t.Fatalf("attachments = %#v", database.Attachments)
	}

	if snapshot.Postgres.State != "running" || snapshot.Postgres.PIDs != 7 {
		t.Fatalf("postgres = %#v", snapshot.Postgres)
	}
}

func TestClientDatabasesNormalizesNullList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"databases":null,"postgres":{"state":"running"},"collectedAt":"2026-09-07T08:00:00Z"}`))
	}))
	defer server.Close()

	snapshot, err := NewClient(server.URL, time.Second).Databases(context.Background())
	if err != nil {
		t.Fatalf("Databases() error = %v", err)
	}
	if snapshot.Databases == nil || len(snapshot.Databases) != 0 {
		t.Fatalf("databases = %#v", snapshot.Databases)
	}
}

func TestClientDatabasesRejectsNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, time.Second).Databases(context.Background()); err == nil {
		t.Fatal("Databases() error = nil")
	}
}

func TestFindDatabase(t *testing.T) {
	snapshot := Snapshot{
		Databases: []Database{
			{ID: "database_a", DisplayName: "A"},
			{ID: "database_b", DisplayName: "B"},
		},
	}
	database, ok := FindDatabase(snapshot, "database_b")
	if !ok || database.DisplayName != "B" {
		t.Fatalf("database = %#v, ok = %v", database, ok)
	}
	if _, ok := FindDatabase(snapshot, "missing"); ok {
		t.Fatal("missing database unexpectedly found")
	}
}

func TestClientGuestDatabasesUsesGuestContractAndDiscardsUnknownFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/guest/databases" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"summary":{"total":2,"showing":1,"hidden":1},
			"databases":[{
				"id":"database_a",
				"displayName":"Shared Database",
				"status":"ready",
				"internalName":"must-not-pass",
				"attachments":[{"consumerRef":"must-not-pass"}]
			}],
			"postgres":{"state":"must-not-pass"}
		}`))
	}))
	defer server.Close()

	result, err := NewClient(server.URL, time.Second).GuestDatabases(context.Background())
	if err != nil {
		t.Fatalf("GuestDatabases() error = %v", err)
	}
	if result.Summary != (GuestSummary{Total: 2, Showing: 1, Hidden: 1}) {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if len(result.Databases) != 1 ||
		result.Databases[0] != (GuestDatabase{
			ID: "database_a", DisplayName: "Shared Database", Status: "ready",
		}) {
		t.Fatalf("databases = %#v", result.Databases)
	}

	encoded, err := json.Marshal(result.Databases[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"id":"database_a","displayName":"Shared Database","status":"ready"}` {
		t.Fatalf("Guest database fields = %s", encoded)
	}
}

func TestClientGuestDatabasesRejectsMalformedOrInvalidResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{`},
		{name: "missing summary", body: `{"databases":[]}`},
		{name: "missing databases", body: `{"summary":{"total":0,"showing":0,"hidden":0}}`},
		{name: "negative count", body: `{"summary":{"total":-1,"showing":0,"hidden":-1},"databases":[]}`},
		{name: "showing mismatch", body: `{"summary":{"total":1,"showing":1,"hidden":0},"databases":[]}`},
		{name: "hidden invariant", body: `{"summary":{"total":3,"showing":1,"hidden":1},"databases":[{"id":"database_a","displayName":"A","status":"ready"}]}`},
		{name: "empty item field", body: `{"summary":{"total":1,"showing":1,"hidden":0},"databases":[{"id":"database_a","displayName":"","status":"ready"}]}`},
		{name: "trailing JSON", body: `{"summary":{"total":0,"showing":0,"hidden":0},"databases":[]} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			if _, err := NewClient(server.URL, time.Second).GuestDatabases(context.Background()); err == nil {
				t.Fatal("GuestDatabases() error = nil")
			}
		})
	}
}

func TestClientGuestDatabasesRejectsNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private upstream detail", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, time.Second).GuestDatabases(context.Background()); err == nil {
		t.Fatal("GuestDatabases() error = nil")
	}
}
