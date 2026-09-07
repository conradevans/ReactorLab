package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	NewHandler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "reactorlab") {
		t.Fatalf("response missing service name: %s", w.Body.String())
	}
}

func TestGuestStatusIsAllowlisted(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/guest/status", nil)
	w := httptest.NewRecorder()

	NewHandler().ServeHTTP(w, r)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if strings.Contains(body, "administrator") || strings.Contains(body, "phase") {
		t.Fatalf("guest response leaked admin fields: %s", body)
	}
}

func TestAdminStatus(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	NewHandler().ServeHTTP(w, r)

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

	NewHandler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
