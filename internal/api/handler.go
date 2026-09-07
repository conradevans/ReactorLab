package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

type Handler struct {
	mux         *http.ServeMux
	frontendDir string
}

func NewHandler(frontendDir string) http.Handler {
	h := &Handler{
		mux:         http.NewServeMux(),
		frontendDir: frontendDir,
	}

	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc("GET /api/v1/status", h.adminStatus)
	h.mux.HandleFunc("GET /api/v1/guest/status", h.guestStatus)
	h.mux.HandleFunc("GET /api/v1/system", h.adminSystem)
	h.mux.HandleFunc("GET /", h.frontend)

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "reactorlab",
		"status":  "ok",
	})
}

func (h *Handler) adminStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":    "reactorlab",
		"apiVersion": "v1",
		"mode":       "administrator",
		"phase":      "foundation",
	})
}

func (h *Handler) guestStatus(w http.ResponseWriter, _ *http.Request) {
	uptime, err := reactorsystem.Uptime()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "uptime_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"uptimeSeconds": uptime,
	})
}

func (h *Handler) adminSystem(w http.ResponseWriter, _ *http.Request) {
	metrics, err := reactorsystem.Collect()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "system_metrics_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (h *Handler) frontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "not_found",
		})
		return
	}

	if h.frontendDir == "" {
		http.NotFound(w, r)
		return
	}

	requestPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	candidate := filepath.Join(h.frontendDir, filepath.FromSlash(requestPath))

	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		http.ServeFile(w, r, candidate)
		return
	}

	indexPath := filepath.Join(h.frontendDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		http.Error(w, "ReactorLab frontend is unavailable", http.StatusServiceUnavailable)
		return
	}

	http.ServeFile(w, r, indexPath)
}
