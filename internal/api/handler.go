package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/conradevans/ReactorLab/internal/accessauth"
	"github.com/conradevans/ReactorLab/internal/history"
	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/observability"
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

type observabilitySource interface {
	QueryHost(context.Context, observability.Range) ([]observability.HostPoint, error)
	QueryTemperature(context.Context, observability.Range) ([]observability.TemperaturePoint, error)
	QueryApplications(context.Context, observability.Range) ([]observability.ApplicationSummary, error)
	QueryApplication(context.Context, string, observability.Range) ([]observability.ApplicationPoint, error)
	QueryServices(context.Context, observability.Range) ([]observability.ServiceSeries, error)
	QueryEvents(context.Context, time.Time, time.Time, int) ([]observability.Event, error)
	LatestRecoveryIncident(context.Context) (*observability.RecoveryIncident, error)
	ListRecoveryIncidents(context.Context, int) ([]observability.RecoveryIncident, error)
	RecoveryIncidentByID(context.Context, string) (*observability.RecoveryIncident, error)
}

type activityIncidentResponse struct {
	LastKnownAliveAt time.Time `json:"lastKnownAliveAt"`
	RecoveredAt      time.Time `json:"recoveredAt"`
	DowntimeSeconds  int64     `json:"downtimeSeconds"`
	Status           string    `json:"status"`
}

type activityEventResponse struct {
	ID         int64                     `json:"id,omitempty"`
	EventID    string                    `json:"eventId,omitempty"`
	OccurredAt time.Time                 `json:"occurredAt"`
	Source     string                    `json:"source"`
	Kind       string                    `json:"kind"`
	Severity   string                    `json:"severity"`
	Subject    string                    `json:"subject"`
	Message    string                    `json:"message"`
	Incident   *activityIncidentResponse `json:"incident,omitempty"`
}

type activityResponse struct {
	Events []activityEventResponse `json:"events"`
}
type systemRecoveryIncidentResponse struct {
	EventID          string    `json:"eventId"`
	LastKnownAliveAt time.Time `json:"lastKnownAliveAt"`
	RecoveredAt      time.Time `json:"recoveredAt"`
	DowntimeSeconds  int64     `json:"downtimeSeconds"`
	Status           string    `json:"status"`
}

type systemRecoveryResponse struct {
	Protection       reactorsystem.RecoveryProtectionState `json:"protection"`
	HistoryAvailable bool                                  `json:"historyAvailable"`
	LastIncident     *systemRecoveryIncidentResponse       `json:"lastIncident"`
}

type adminSystemResponse struct {
	reactorsystem.Metrics
	Recovery systemRecoveryResponse `json:"recovery"`
}

type Handler struct {
	mux                               *http.ServeMux
	frontendDir                       string
	miniDeploy                        deploymentMetricsSource
	miniBase                          databaseMetricsSource
	guestMiniDeploy                   guestDeploymentSource
	guestMiniBase                     guestDatabaseSource
	activity                          activitySource
	observability                     observabilitySource
	miniAIDeploy                      miniAIDeploymentSource
	miniAIBase                        miniAIDatabaseSource
	access                            accessauth.TokenValidator
	now                               func() time.Time
	collectSystem                     func() (reactorsystem.Metrics, error)
	inspectHardwareWatchdogProtection func(context.Context) (reactorsystem.HardwareWatchdogProtectionState, error)
	inspectRTCRecoveryProtection      func(context.Context) (reactorsystem.RTCRecoveryProtectionState, error)
}

func NewHandler(frontendDir string) http.Handler {
	return newProductionHandler(frontendDir, nil, nil)
}

func NewHandlerWithHistory(
	frontendDir string,
	activity activitySource,
) http.Handler {
	return newProductionHandler(frontendDir, activity, nil)
}

func NewHandlerWithHistoryAndAccess(
	frontendDir string,
	activity activitySource,
	access accessauth.TokenValidator,
) http.Handler {
	return newProductionHandler(frontendDir, activity, access)
}

func NewHandlerWithHistoryAccessAndObservability(
	frontendDir string,
	activity activitySource,
	access accessauth.TokenValidator,
	source observabilitySource,
	miniDeployURL string,
	miniBaseURL string,
) http.Handler {
	handler := newHandlerWithServiceClients(
		frontendDir, activity, access,
		newServiceClients(miniDeployURL, minideploy.DefaultGuestBaseURL, miniBaseURL),
	)
	h := handler.(*Handler)
	h.observability = source
	h.registerObservabilityRoutes()
	return h
}

func newProductionHandler(
	frontendDir string,
	activity activitySource,
	access accessauth.TokenValidator,
) http.Handler {
	return newHandlerWithServiceClients(
		frontendDir,
		activity,
		access,
		newProductionServiceClients(),
	)
}

type serviceClients struct {
	privateMiniDeploy *minideploy.Client
	guestMiniDeploy   *minideploy.Client
	miniBase          *minibase.Client
}

func newProductionServiceClients() serviceClients {
	return newServiceClients(
		minideploy.DefaultBaseURL,
		minideploy.DefaultGuestBaseURL,
		minibase.DefaultBaseURL,
	)
}

func newServiceClients(
	privateMiniDeployBaseURL string,
	guestMiniDeployBaseURL string,
	miniBaseBaseURL string,
) serviceClients {
	return serviceClients{
		privateMiniDeploy: minideploy.NewClient(
			privateMiniDeployBaseURL,
			5*time.Second,
		),
		guestMiniDeploy: minideploy.NewClient(
			guestMiniDeployBaseURL,
			5*time.Second,
		),
		miniBase: minibase.NewClient(
			miniBaseBaseURL,
			5*time.Second,
		),
	}
}

func newHandlerWithServiceClients(
	frontendDir string,
	activity activitySource,
	access accessauth.TokenValidator,
	clients serviceClients,
) http.Handler {
	return newHandlerWithAllSourcesAndAccessAndGuestSources(
		frontendDir,
		clients.privateMiniDeploy,
		clients.miniBase,
		activity,
		access,
		clients.guestMiniDeploy,
		clients.miniBase,
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
	return newHandlerWithAllSourcesAndAccess(
		frontendDir,
		miniDeploy,
		miniBase,
		activity,
		nil,
	)
}

func newHandlerWithAllSourcesAndAccess(
	frontendDir string,
	miniDeploy deploymentMetricsSource,
	miniBase databaseMetricsSource,
	activity activitySource,
	access accessauth.TokenValidator,
) http.Handler {
	return newHandlerWithAllSourcesAndAccessAndGuestSources(
		frontendDir,
		miniDeploy,
		miniBase,
		activity,
		access,
		nil,
		nil,
	)
}

func newHandlerWithGuestSources(
	frontendDir string,
	miniDeploy guestDeploymentSource,
	miniBase guestDatabaseSource,
) http.Handler {
	return newHandlerWithAllSourcesAndAccessAndGuestSources(
		frontendDir,
		nil,
		nil,
		nil,
		nil,
		miniDeploy,
		miniBase,
	)
}

func newHandlerWithAllSourcesAndAccessAndGuestSources(
	frontendDir string,
	miniDeploy deploymentMetricsSource,
	miniBase databaseMetricsSource,
	activity activitySource,
	access accessauth.TokenValidator,
	guestMiniDeploy guestDeploymentSource,
	guestMiniBase guestDatabaseSource,
) http.Handler {
	h := &Handler{
		mux:                               http.NewServeMux(),
		frontendDir:                       frontendDir,
		miniDeploy:                        miniDeploy,
		miniBase:                          miniBase,
		guestMiniDeploy:                   guestMiniDeploy,
		guestMiniBase:                     guestMiniBase,
		activity:                          activity,
		access:                            access,
		now:                               func() time.Time { return time.Now().UTC() },
		collectSystem:                     reactorsystem.Collect,
		inspectHardwareWatchdogProtection: reactorsystem.InspectHardwareWatchdogProtection,
		inspectRTCRecoveryProtection:      reactorsystem.InspectRTCRecoveryProtection,
	}
	if source, ok := miniDeploy.(miniAIDeploymentSource); ok {
		h.miniAIDeploy = source
	}
	if source, ok := miniBase.(miniAIDatabaseSource); ok {
		h.miniAIBase = source
	}

	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc("GET /api/v1/status", h.adminStatus)
	h.mux.HandleFunc("GET /api/v1/session", h.adminSession)
	h.mux.HandleFunc("GET /api/v1/guest/status", h.guestStatus)
	h.mux.HandleFunc("GET /api/v1/guest/resources", h.guestResources)
	h.mux.HandleFunc("GET /api/v1/system", h.adminSystem)
	h.mux.HandleFunc("GET /api/v1/deployments", h.adminDeployments)
	h.mux.HandleFunc("GET /api/v1/deployments/{app}", h.adminDeployment)
	h.mux.HandleFunc("GET /api/v1/databases", h.adminDatabases)
	h.mux.HandleFunc("GET /api/v1/databases/{id}", h.adminDatabase)
	h.mux.HandleFunc("GET /api/v1/activity", h.adminActivity)
	h.registerMiniAIRoutes()
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

func (h *Handler) adminSession(w http.ResponseWriter, r *http.Request) {
	if h.access == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"mode": "local",
		})
		return
	}

	rawToken := strings.TrimSpace(
		r.Header.Get(accessauth.AccessJWTHeader),
	)
	if rawToken == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "access_authentication_required",
		})
		return
	}

	identity, err := h.access.Validate(r.Context(), rawToken)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error": "access_denied",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"mode":  "access",
		"email": identity.Email,
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

func (h *Handler) adminSystem(w http.ResponseWriter, r *http.Request) {
	collectSystem := h.collectSystem
	if collectSystem == nil {
		collectSystem = reactorsystem.Collect
	}
	metrics, err := collectSystem()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "system_metrics_unavailable",
		})
		return
	}

	inspectHardware := h.inspectHardwareWatchdogProtection
	if inspectHardware == nil {
		inspectHardware = reactorsystem.InspectHardwareWatchdogProtection
	}
	hardwareWatchdog, hardwareErr := inspectHardware(r.Context())
	if hardwareErr != nil ||
		!reactorsystem.ValidRecoveryProtectionState(hardwareWatchdog.State) {
		hardwareWatchdog = reactorsystem.HardwareWatchdogProtectionState{
			State: reactorsystem.RecoveryProtectionUnavailable,
		}
	}

	inspectRTC := h.inspectRTCRecoveryProtection
	if inspectRTC == nil {
		inspectRTC = reactorsystem.InspectRTCRecoveryProtection
	}
	rtc, rtcErr := inspectRTC(r.Context())
	if rtcErr != nil || !reactorsystem.ValidRecoveryProtectionState(rtc.State) {
		rtc = reactorsystem.RTCRecoveryProtectionState{
			State: reactorsystem.RecoveryProtectionUnavailable,
		}
	}

	protection := reactorsystem.RecoveryProtectionState{
		State: reactorsystem.AggregateRecoveryProtectionState(
			hardwareWatchdog.State,
			rtc.State,
		),
		HardwareWatchdog: hardwareWatchdog,
		RTC:              rtc,
	}
	recovery := systemRecoveryResponse{Protection: protection}
	if h.observability != nil {
		incident, incidentErr := h.observability.LatestRecoveryIncident(r.Context())
		if incidentErr == nil {
			recovery.HistoryAvailable = true
			if incident != nil {
				recovery.LastIncident = &systemRecoveryIncidentResponse{
					EventID:          incident.EventID,
					LastKnownAliveAt: incident.LastKnownAliveAt,
					RecoveredAt:      incident.RecoveredAt,
					DowntimeSeconds:  incident.DowntimeSeconds,
					Status:           incident.Status,
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, adminSystemResponse{
		Metrics:  metrics,
		Recovery: recovery,
	})
}

const activityResultLimit = 100

var recoveryActivityEventID = regexp.MustCompile(`^[0-9a-f]{64}$`)

func requestedRecoveryEventID(r *http.Request) (string, bool) {
	values, present := r.URL.Query()["event"]
	if !present {
		return "", true
	}
	if len(values) != 1 || !recoveryActivityEventID.MatchString(values[0]) {
		return "", false
	}
	return values[0], true
}

func validRecoveryActivityIncident(incident observability.RecoveryIncident) bool {
	return recoveryActivityEventID.MatchString(incident.EventID) &&
		!incident.LastKnownAliveAt.IsZero() && !incident.RecoveredAt.IsZero() &&
		incident.DowntimeSeconds >= 0 && incident.Status == "recovered"
}

func recoveryActivityResponse(incident observability.RecoveryIncident) activityEventResponse {
	return activityEventResponse{
		EventID:    incident.EventID,
		OccurredAt: incident.RecoveredAt.UTC(),
		Source:     "reactorlab",
		Kind:       "unexpected_shutdown_recovery",
		Severity:   "warning",
		Subject:    "Unexpected shutdown detected",
		Message:    "Dell host recovered after an unexpected shutdown.",
		Incident: &activityIncidentResponse{
			LastKnownAliveAt: incident.LastKnownAliveAt.UTC(),
			RecoveredAt:      incident.RecoveredAt.UTC(),
			DowntimeSeconds:  incident.DowntimeSeconds,
			Status:           incident.Status,
		},
	}
}

func sortActivityResponses(events []activityEventResponse) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].OccurredAt.Equal(events[j].OccurredAt) {
			if events[i].EventID != events[j].EventID {
				return events[i].EventID > events[j].EventID
			}
			return events[i].ID > events[j].ID
		}
		return events[i].OccurredAt.After(events[j].OccurredAt)
	})
}

func activityContainsEventID(events []activityEventResponse, eventID string) bool {
	for _, event := range events {
		if event.EventID == eventID {
			return true
		}
	}
	return false
}

func (h *Handler) adminActivity(w http.ResponseWriter, r *http.Request) {
	requestedEventID, valid := requestedRecoveryEventID(r)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_event",
		})
		return
	}
	if h.activity == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "activity_unavailable",
		})
		return
	}

	legacyEvents, err := h.activity.ListActivity(r.Context(), activityResultLimit)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "activity_unavailable",
		})
		return
	}

	response := make([]activityEventResponse, 0, len(legacyEvents)+activityResultLimit)
	for _, event := range legacyEvents {
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

	var requestedEvent *activityEventResponse
	seenRecovery := make(map[string]struct{})
	appendRecovery := func(incident observability.RecoveryIncident) {
		if !validRecoveryActivityIncident(incident) {
			return
		}
		if _, seen := seenRecovery[incident.EventID]; seen {
			return
		}
		seenRecovery[incident.EventID] = struct{}{}
		event := recoveryActivityResponse(incident)
		response = append(response, event)
		if incident.EventID == requestedEventID {
			requestedCopy := event
			requestedEvent = &requestedCopy
		}
	}

	if h.observability != nil {
		incidents, incidentErr := h.observability.ListRecoveryIncidents(
			r.Context(),
			activityResultLimit,
		)
		if incidentErr == nil {
			for _, incident := range incidents {
				appendRecovery(incident)
			}
		}
		if requestedEventID != "" && requestedEvent == nil {
			incident, lookupErr := h.observability.RecoveryIncidentByID(
				r.Context(),
				requestedEventID,
			)
			if lookupErr == nil && incident != nil && incident.EventID == requestedEventID {
				appendRecovery(*incident)
			}
		}
	}

	sortActivityResponses(response)
	if len(response) > activityResultLimit {
		response = response[:activityResultLimit]
	}
	if requestedEvent != nil && !activityContainsEventID(response, requestedEventID) {
		if len(response) == activityResultLimit {
			response = response[:activityResultLimit-1]
		}
		response = append(response, *requestedEvent)
		sortActivityResponses(response)
	}

	writeJSON(w, http.StatusOK, activityResponse{Events: response})
}

func (h *Handler) adminDeployments(w http.ResponseWriter, r *http.Request) {
	databaseResults := h.databaseSnapshotForLinksAsync(r.Context())
	snapshot, err := h.miniDeploy.Deployments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "deployment_metrics_unavailable",
		})
		return
	}

	databaseResult := <-databaseResults
	databaseSnapshot := databaseResult.snapshot
	relationshipsAvailable := databaseResult.available

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
	deploymentResults := h.deploymentSnapshotForLinksAsync(r.Context())
	snapshot, err := h.miniBase.Databases(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "database_metrics_unavailable",
		})
		return
	}

	deploymentResult := <-deploymentResults
	deploymentSnapshot := deploymentResult.snapshot
	relationshipsAvailable := deploymentResult.available

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
