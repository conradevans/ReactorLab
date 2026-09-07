package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	NewHandler("").ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "reactorlab") {
		t.Fatalf("response missing service name: %s", w.Body.String())
	}
}

func TestGuestStatusIsMinimal(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/guest/status", nil)
	w := httptest.NewRecorder()

	NewHandler("").ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if len(body) != 2 {
		t.Fatalf("guest response contains unexpected fields: %#v", body)
	}
	if body["status"] != "ok" {
		t.Fatalf("guest status = %#v", body["status"])
	}
	uptime, ok := body["uptimeSeconds"].(float64)
	if !ok || uptime < 0 {
		t.Fatalf("invalid guest uptime: %#v", body["uptimeSeconds"])
	}
}

func TestGuestSystemEndpointDoesNotExist(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/guest/system", nil)
	w := httptest.NewRecorder()

	NewHandler("").ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAdminStatus(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	NewHandler("").ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "administrator") {
		t.Fatalf("response missing administrator mode: %s", w.Body.String())
	}
}

func TestUnknownAPIIs404(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/not-real", nil)
	w := httptest.NewRecorder()

	NewHandler("").ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestFrontendServesSPAForApplicationRoutes(t *testing.T) {
	dir := t.TempDir()
	index := []byte("<html><body>reactorlab-dashboard</body></html>")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), index, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, route := range []string{"/", "/guest", "/admin", "/admin/system"} {
		t.Run(route, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, route, nil)
			w := httptest.NewRecorder()

			NewHandler(dir).ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			if !strings.Contains(w.Body.String(), "reactorlab-dashboard") {
				t.Fatalf("route %s did not receive SPA index", route)
			}
		})
	}
}
