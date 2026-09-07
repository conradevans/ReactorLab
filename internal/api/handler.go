package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

type deploymentMetricsSource interface {
	Deployments(context.Context) (minideploy.Snapshot, error)
}

type databaseMetricsSource interface {
	Databases(context.Context) (minibase.Snapshot, error)
}

type activitySource interface {
	ListActivity(context.Context, int) ([]history.ActivityEvent, error)
}

type activityEventResponse struct {
	ID         int64     `json:"id"`
	OccurredAt time.Time `json:"occurredAt"`
	Source     string    `json:"source"`
	Kind       string    `json:"kind"`
	Severity   string    `json:"severity"`
	Subject    string    `json:"subject"`
	Message    string    `json:"message"`
}

type activityResponse struct {
	Events []activityEventResponse `json:"events"`
}

type Handler struct {
	mux         *http.ServeMux
	frontendDir string
	miniDeploy  deploymentMetricsSource
	miniBase    databaseMetricsSource
	activity    activitySource
}

func NewHandler(frontendDir string) http.Handler {
	return newHandlerWithAllSources(
		frontendDir,
		minideploy.NewClient(minideploy.DefaultBaseURL, 5*time.Second),
		minibase.NewClient(minibase.DefaultBaseURL, 5*time.Second),
		nil,
	)
}

func NewHandlerWithHistory(
	frontendDir string,
	activity activitySource,
) http.Handler {
	return newHandlerWithAllSources(
		frontendDir,
		minideploy.NewClient(minideploy.DefaultBaseURL, 5*time.Second),
		minibase.NewClient(minibase.DefaultBaseURL, 5*time.Second),
		activity,
	)
}

func newHandler(
	frontendDir string,
	miniDeploy deploymentMetricsSource,
) http.Handler {
	return newHandlerWithSources(frontendDir, miniDeploy, nil)
}

func newHandlerWithSources(
	frontendDir string,
	miniDeploy deploymentMetricsSource,
	miniBase databaseMetricsSource,
) http.Handler {
	return newHandlerWithAllSources(
		frontendDir,
		miniDeploy,
		miniBase,
		nil,
	)
}

func newHandlerWithAllSources(
	frontendDir string,
	miniDeploy deploymentMetricsSource,
	miniBase databaseMetricsSource,
	activity activitySource,
) http.Handler {
	h := &Handler{
		mux:         http.NewServeMux(),
		frontendDir: frontendDir,
		miniDeploy:  miniDeploy,
		miniBase:    miniBase,
		activity:    activity,
	}

	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc("GET /api/v1/status", h.adminStatus)
	h.mux.HandleFunc("GET /api/v1/guest/status", h.guestStatus)
	h.mux.HandleFunc("GET /api/v1/system", h.adminSystem)
	h.mux.HandleFunc("GET /api/v1/deployments", h.adminDeployments)
	h.mux.HandleFunc("GET /api/v1/deployments/{app}", h.adminDeployment)
	h.mux.HandleFunc("GET /api/v1/databases", h.adminDatabases)
	h.mux.HandleFunc("GET /api/v1/databases/{id}", h.adminDatabase)
	h.mux.HandleFunc("GET /api/v1/activity", h.adminActivity)
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

func (h *Handler) adminActivity(w http.ResponseWriter, r *http.Request) {
	if h.activity == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "activity_unavailable",
		})
		return
	}

	events, err := h.activity.ListActivity(r.Context(), 100)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "activity_unavailable",
		})
		return
	}

	response := make([]activityEventResponse, 0, len(events))
	for _, event := range events {
		response = append(response, activityEventResponse{
			ID:         event.ID,
			OccurredAt: event.OccurredAt,
			Source:     event.Source,
			Kind:       event.Kind,
			Severity:   event.Severity,
			Subject:    event.Subject,
			Message:    event.Message,
		})
	}

	writeJSON(w, http.StatusOK, activityResponse{Events: response})
}

func (h *Handler) adminDeployments(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.miniDeploy.Deployments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "deployment_metrics_unavailable",
		})
		return
	}

	databaseSnapshot, relationshipsAvailable :=
		h.databaseSnapshotForLinks(r.Context())

	deployments := make(
		[]deploymentResponse,
		0,
		len(snapshot.Deployments),
	)
	for _, deployment := range snapshot.Deployments {
		link := unavailableDatabaseLink()
		if relationshipsAvailable {
			link = databaseLinkForDeployment(
				deployment.App,
				databaseSnapshot,
			)
		}
		deployments = append(
			deployments,
			projectDeployment(deployment, link),
		)
	}

	writeJSON(w, http.StatusOK, deploymentsResponse{
		Deployments: deployments,
		CollectedAt: snapshot.CollectedAt,
	})
}

func (h *Handler) adminDeployment(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.miniDeploy.Deployments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "deployment_metrics_unavailable",
		})
		return
	}

	deployment, ok := minideploy.FindDeployment(
		snapshot,
		r.PathValue("app"),
	)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "deployment_not_found",
		})
		return
	}

	link := unavailableDatabaseLink()
	if databaseSnapshot, ok := h.databaseSnapshotForLinks(
		r.Context(),
	); ok {
		link = databaseLinkForDeployment(
			deployment.App,
			databaseSnapshot,
		)
	}

	writeJSON(
		w,
		http.StatusOK,
		projectDeployment(deployment, link),
	)
}

func (h *Handler) adminDatabases(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.miniBase.Databases(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "database_metrics_unavailable",
		})
		return
	}

	deploymentSnapshot, relationshipsAvailable :=
		h.deploymentSnapshotForLinks(r.Context())

	databases := make(
		[]databaseResponse,
		0,
		len(snapshot.Databases),
	)
	for _, database := range snapshot.Databases {
		link := unavailableDeploymentLink()
		if relationshipsAvailable {
			link = deploymentLinkForDatabase(
				database,
				deploymentSnapshot,
			)
		}
		databases = append(
			databases,
			projectDatabase(database, link),
		)
	}

	writeJSON(w, http.StatusOK, databasesResponse{
		Databases:   databases,
		Postgres:    snapshot.Postgres,
		CollectedAt: snapshot.CollectedAt,
	})
}

func (h *Handler) adminDatabase(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.miniBase.Databases(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "database_metrics_unavailable",
		})
		return
	}

	database, ok := minibase.FindDatabase(
		snapshot,
		r.PathValue("id"),
	)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "database_not_found",
		})
		return
	}

	link := unavailableDeploymentLink()
	if deploymentSnapshot, ok := h.deploymentSnapshotForLinks(
		r.Context(),
	); ok {
		link = deploymentLinkForDatabase(
			database,
			deploymentSnapshot,
		)
	}

	writeJSON(
		w,
		http.StatusOK,
		projectDatabase(database, link),
	)
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

	requestPath := strings.TrimPrefix(
		path.Clean("/"+r.URL.Path),
		"/",
	)
	candidate := filepath.Join(
		h.frontendDir,
		filepath.FromSlash(requestPath),
	)

	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		http.ServeFile(w, r, candidate)
		return
	}

	indexPath := filepath.Join(h.frontendDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		http.Error(
			w,
			"ReactorLab frontend is unavailable",
			http.StatusServiceUnavailable,
		)
		return
	}

	http.ServeFile(w, r, indexPath)
}
