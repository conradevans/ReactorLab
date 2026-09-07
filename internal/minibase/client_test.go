package minibase

import (
	"context"
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
