package minibase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestActivityReadsManagementHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/activity" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"databaseId":"database_a","databaseDisplayName":"Primary","type":"backup_create","outcome":"success","source":"admin","detail":"Backup created","createdAt":"2026-09-17T12:00:00Z"}]`))
	}))
	defer server.Close()

	events, err := NewClient(server.URL, time.Second).Activity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "backup_create" || events[0].DatabaseID != "database_a" {
		t.Fatalf("events = %#v", events)
	}
}

func TestActivityRejectsMalformedSafeContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"type":"backup_create"}]`))
	}))
	defer server.Close()
	if _, err := NewClient(server.URL, time.Second).Activity(context.Background()); err == nil {
		t.Fatal("expected invalid activity contract error")
	}
}
