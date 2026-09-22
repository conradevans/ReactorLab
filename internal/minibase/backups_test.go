package minibase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDatabaseBackupsReturnsOnlyValidatedSafeFields(t *testing.T) {
	databaseID := "database_11111111111111111111111111111111"
	backupID := "backup_22222222222222222222222222222222"
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet ||
				request.URL.Path != "/api/v1/databases/"+databaseID+"/backups" {

				t.Fatalf("request = %s %s", request.Method, request.URL.Path)
			}
			_, _ = response.Write([]byte(`[{
				"id":"` + backupID + `",
				"databaseId":"` + databaseID + `",
				"databaseDisplayName":"Production",
				"kind":"automatic",
				"status":"ready",
				"sizeBytes":4096,
				"createdAt":"2026-09-22T10:00:00Z",
				"completedAt":"2026-09-22T10:01:00Z",
				"path":"/srv/minibase/private/backup.tar",
				"password":"MUST_NOT_PASS"
			}]`))
		},
	))
	defer server.Close()

	backups, err := NewClient(
		server.URL,
		time.Second,
	).DatabaseBackups(context.Background(), databaseID)
	if err != nil {
		t.Fatalf("DatabaseBackups() error = %v", err)
	}
	if len(backups) != 1 || backups[0].ID != backupID ||
		backups[0].SizeBytes != 4096 ||
		backups[0].CompletedAt == nil {

		t.Fatalf("backups = %#v", backups)
	}
	encoded, err := json.Marshal(backups)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"path", "/srv/minibase", "password", "MUST_NOT_PASS",
		"databaseDisplayName",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("safe backup leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestDatabaseBackupsRejectsMalformedResponses(t *testing.T) {
	databaseID := "database_11111111111111111111111111111111"
	valid := `{
		"id":"backup_22222222222222222222222222222222",
		"databaseId":"` + databaseID + `",
		"databaseDisplayName":"Production",
		"kind":"manual",
		"status":"ready",
		"sizeBytes":10,
		"createdAt":"2026-09-22T10:00:00Z",
		"completedAt":"2026-09-22T10:01:00Z"
	}`
	for _, body := range []string{
		`[{"id":"invalid","databaseId":"` + databaseID + `","databaseDisplayName":"Production","kind":"manual","status":"ready","sizeBytes":10,"createdAt":"2026-09-22T10:00:00Z","completedAt":"2026-09-22T10:01:00Z"}]`,
		`[{"id":"backup_22222222222222222222222222222222","databaseId":"database_33333333333333333333333333333333","databaseDisplayName":"Production","kind":"manual","status":"ready","sizeBytes":10,"createdAt":"2026-09-22T10:00:00Z","completedAt":"2026-09-22T10:01:00Z"}]`,
		`[{"id":"backup_22222222222222222222222222222222","databaseId":"` + databaseID + `","databaseDisplayName":"Production","kind":"unsafe","status":"ready","sizeBytes":10,"createdAt":"2026-09-22T10:00:00Z","completedAt":"2026-09-22T10:01:00Z"}]`,
		`[{"id":"backup_22222222222222222222222222222222","databaseId":"` + databaseID + `","databaseDisplayName":"Production","kind":"manual","status":"ready","sizeBytes":-1,"createdAt":"2026-09-22T10:00:00Z","completedAt":"2026-09-22T10:01:00Z"}]`,
		`[` + valid + `] {}`,
		`[]` + strings.Repeat(" ", maxBackupResponseBytes),
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(
				func(response http.ResponseWriter, _ *http.Request) {
					_, _ = response.Write([]byte(body))
				},
			))
			defer server.Close()
			if _, err := NewClient(
				server.URL,
				time.Second,
			).DatabaseBackups(context.Background(), databaseID); err == nil {

				t.Fatal("DatabaseBackups() error = nil")
			}
		})
	}
}

func TestDatabaseBackupsEnforcesLifecycleInvariants(t *testing.T) {
	databaseID := "database_11111111111111111111111111111111"
	createdAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	completedAt := createdAt.Add(time.Minute)
	beforeCreatedAt := createdAt.Add(-time.Minute)
	tests := []struct {
		name        string
		status      string
		sizeBytes   int64
		completedAt *time.Time
		wantError   bool
	}{
		{name: "valid creating", status: "creating"},
		{
			name:        "valid ready",
			status:      "ready",
			sizeBytes:   10,
			completedAt: &completedAt,
		},
		{
			name:        "valid error",
			status:      "error",
			completedAt: &completedAt,
		},
		{
			name:        "creating with completion",
			status:      "creating",
			completedAt: &completedAt,
			wantError:   true,
		},
		{
			name:      "creating with positive size",
			status:    "creating",
			sizeBytes: 10,
			wantError: true,
		},
		{name: "ready without completion", status: "ready", sizeBytes: 10, wantError: true},
		{
			name:        "ready with zero size",
			status:      "ready",
			completedAt: &completedAt,
			wantError:   true,
		},
		{name: "error without completion", status: "error", wantError: true},
		{
			name:        "error with positive size",
			status:      "error",
			sizeBytes:   10,
			completedAt: &completedAt,
			wantError:   true,
		},
		{
			name:        "completion before creation",
			status:      "ready",
			sizeBytes:   10,
			completedAt: &beforeCreatedAt,
			wantError:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal([]backupWire{{
				ID:                  "backup_22222222222222222222222222222222",
				DatabaseID:          databaseID,
				DatabaseDisplayName: "Production",
				Kind:                "manual",
				Status:              test.status,
				SizeBytes:           test.sizeBytes,
				CreatedAt:           createdAt,
				CompletedAt:         test.completedAt,
			}})
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(
				func(response http.ResponseWriter, _ *http.Request) {
					_, _ = response.Write(body)
				},
			))
			defer server.Close()

			backups, err := NewClient(
				server.URL,
				time.Second,
			).DatabaseBackups(context.Background(), databaseID)
			if test.wantError {
				if err == nil {
					t.Fatalf("DatabaseBackups() = %#v, want error", backups)
				}
				return
			}
			if err != nil || len(backups) != 1 ||
				backups[0].Status != test.status ||
				backups[0].SizeBytes != test.sizeBytes ||
				(backups[0].CompletedAt == nil) != (test.completedAt == nil) {

				t.Fatalf("DatabaseBackups() = %#v, %v", backups, err)
			}
		})
	}
}
