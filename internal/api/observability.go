package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/conradevans/ReactorLab/internal/observability"
)

type observabilityEnvelope struct {
	Range string    `json:"range"`
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	Data  any       `json:"data"`
}

func (h *Handler) registerObservabilityRoutes() {
	h.mux.HandleFunc("GET /api/v1/observability/host", h.adminObservabilityHost)
	h.mux.HandleFunc("GET /api/v1/observability/temperature", h.adminObservabilityTemperature)
	h.mux.HandleFunc("GET /api/v1/observability/apps", h.adminObservabilityApplications)
	h.mux.HandleFunc("GET /api/v1/observability/apps/{id}", h.adminObservabilityApplication)
	h.mux.HandleFunc("GET /api/v1/observability/services", h.adminObservabilityServices)
	h.mux.HandleFunc("GET /api/v1/observability/events", h.adminObservabilityEvents)
}

func newHandlerWithObservability(frontendDir string, source observabilitySource) http.Handler {
	h := newProductionHandler(frontendDir, nil, nil).(*Handler)
	h.observability = source
	h.registerObservabilityRoutes()
	return h
}

func (h *Handler) observabilityRange(w http.ResponseWriter, r *http.Request) (observability.Range, bool) {
	if h.observability == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "observability_unavailable"})
		return observability.Range{}, false
	}
	window, err := observability.ResolveRange(r.URL.Query().Get("range"), time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_range",
			"allowed": []string{"15m", "1h", "6h", "24h", "7d"},
		})
		return observability.Range{}, false
	}
	return window, true
}

func (h *Handler) writeObservability(w http.ResponseWriter, window observability.Range, data any, err error) {
	if err != nil {
		status, code := http.StatusServiceUnavailable, "observability_unavailable"
		if errors.Is(err, observability.ErrInvalidRange) {
			status, code = http.StatusBadRequest, "invalid_range"
		}
		writeJSON(w, status, map[string]any{"error": code})
		return
	}
	writeJSON(w, http.StatusOK, observabilityEnvelope{
		Range: window.Name, From: window.From, To: window.To, Data: data,
	})
}

func (h *Handler) adminObservabilityHost(w http.ResponseWriter, r *http.Request) {
	window, ok := h.observabilityRange(w, r)
	if !ok {
		return
	}
	points, err := h.observability.QueryHost(r.Context(), window)
	h.writeObservability(w, window, map[string]any{"points": points}, err)
}

func (h *Handler) adminObservabilityTemperature(w http.ResponseWriter, r *http.Request) {
	window, ok := h.observabilityRange(w, r)
	if !ok {
		return
	}
	points, err := h.observability.QueryTemperature(r.Context(), window)
	h.writeObservability(w, window, map[string]any{"points": points}, err)
}

func (h *Handler) adminObservabilityApplications(w http.ResponseWriter, r *http.Request) {
	window, ok := h.observabilityRange(w, r)
	if !ok {
		return
	}
	applications, err := h.observability.QueryApplications(r.Context(), window)
	h.writeObservability(w, window, map[string]any{"applications": applications}, err)
}

func (h *Handler) adminObservabilityApplication(w http.ResponseWriter, r *http.Request) {
	window, ok := h.observabilityRange(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	points, err := h.observability.QueryApplication(r.Context(), id, window)
	h.writeObservability(w, window, map[string]any{"id": id, "points": points}, err)
}

func (h *Handler) adminObservabilityServices(w http.ResponseWriter, r *http.Request) {
	window, ok := h.observabilityRange(w, r)
	if !ok {
		return
	}
	services, err := h.observability.QueryServices(r.Context(), window)
	h.writeObservability(w, window, map[string]any{"services": services}, err)
}

func (h *Handler) adminObservabilityEvents(w http.ResponseWriter, r *http.Request) {
	window, ok := h.observabilityRange(w, r)
	if !ok {
		return
	}
	events, err := h.observability.QueryEvents(r.Context(), window.From, window.To, 200)
	h.writeObservability(w, window, map[string]any{"events": events}, err)
}
