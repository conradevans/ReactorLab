package minibase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidatedDatabasesRequiresCompleteSingleJSONResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "valid empty snapshot",
			body: `{
				"databases":[],
				"postgres":{"state":"running"},
				"collectedAt":"2026-09-22T18:00:00Z"
			}`,
		},
		{
			name: "missing databases",
			body: `{
				"postgres":{"state":"running"},
				"collectedAt":"2026-09-22T18:00:00Z"
			}`,
		},
		{
			name: "null databases",
			body: `{
				"databases":null,
				"postgres":{"state":"running"},
				"collectedAt":"2026-09-22T18:00:00Z"
			}`,
		},
		{
			name: "missing postgres",
			body: `{
				"databases":[],
				"collectedAt":"2026-09-22T18:00:00Z"
			}`,
		},
		{
			name: "missing collection time",
			body: `{
				"databases":[],
				"postgres":{"state":"running"}
			}`,
		},
		{
			name: "trailing JSON",
			body: `{
				"databases":[],
				"postgres":{"state":"running"},
				"collectedAt":"2026-09-22T18:00:00Z"
			} {}`,
		},
		{
			name: "oversized response",
			body: `{
				"databases":[],
				"postgres":{"state":"running"},
				"collectedAt":"2026-09-22T18:00:00Z"
			}` + strings.Repeat(" ", maxDatabaseResponseBytes),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(
				func(response http.ResponseWriter, _ *http.Request) {
					_, _ = response.Write([]byte(test.body))
				},
			))
			defer server.Close()

			snapshot, err := NewClient(
				server.URL,
				time.Second,
			).ValidatedDatabases(context.Background())
			if test.name == "valid empty snapshot" {
				if err != nil || snapshot.Databases == nil {
					t.Fatalf(
						"ValidatedDatabases() = %#v, %v",
						snapshot,
						err,
					)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidatedDatabases() error = nil")
			}
		})
	}
}
