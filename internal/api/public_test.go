package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicHandlerAllowlist(t *testing.T) {
	frontend := t.TempDir()
	if err := os.MkdirAll(filepath.Join(frontend, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.html":    "<html>reactorlab</html>",
		"favicon.svg":   "<svg></svg>",
		"assets/app.js": "console.log('ok')",
	} {
		if err := os.WriteFile(filepath.Join(frontend, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	handler := NewPublicHandler(frontend)

	allowed := []string{
		"/health",
		"/",
		"/guest",
		"/guest/",
		"/api/v1/guest/status",
		"/favicon.svg",
		"/assets/app.js",
	}
	for _, route := range allowed {
		t.Run("allow "+route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: got %d, want 200; body=%s", route, rec.Code, rec.Body.String())
			}
		})
	}

	blocked := []string{
		"/admin",
		"/admin/system",
		"/admin/deployments",
		"/admin/databases",
		"/admin/activity",
		"/api/v1/status",
		"/api/v1/session",
		"/api/v1/system",
		"/api/v1/deployments",
		"/api/v1/databases",
		"/api/v1/activity",
		"/api/v1/guest/system",
		"/api/v1/guest/deployments",
		"/api/v1/guest/databases",
		"/api/v1/guest/activity",
		"/not-a-public-route",
	}
	for _, route := range blocked {
		t.Run("block "+route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s: got %d, want 404; body=%s", route, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPublicHandlerRejectsWrites(t *testing.T) {
	handler := NewPublicHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guest/status", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}
