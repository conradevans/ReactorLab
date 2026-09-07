package api

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	mux *http.ServeMux
}

func NewHandler() http.Handler {
	h := &Handler{mux: http.NewServeMux()}

	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc("GET /api/v1/status", h.adminStatus)
	h.mux.HandleFunc("GET /api/v1/guest/status", h.guestStatus)

	return h.mux
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
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "reactorlab",
		"status":  "ok",
	})
}
