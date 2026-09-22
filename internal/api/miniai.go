package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/conradevans/ReactorLab/internal/intelligence"
	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
	"github.com/conradevans/ReactorLab/internal/observability"
	reactorsystem "github.com/conradevans/ReactorLab/internal/system"
)

const (
	miniAIRequestTimeout = 5 * time.Second
	miniAIRecoveryLimit  = 20
)

var observabilityResourceIDPattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`,
)

type miniAIDeploymentSource interface {
	DeploymentMetadata(context.Context) ([]minideploy.DeploymentMetadata, error)
	DeploymentHistory(context.Context, string) (minideploy.DeploymentHistory, error)
}

type miniAIDatabaseSource interface {
	ValidatedDatabases(context.Context) (minibase.Snapshot, error)
	DatabaseBackups(context.Context, string) ([]minibase.Backup, error)
}

func (h *Handler) registerMiniAIRoutes() {
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/overview",
		h.miniAIOverview,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/observability/host",
		h.miniAIObservabilityHost,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/observability/temperature",
		h.miniAIObservabilityTemperature,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/observability/applications",
		h.miniAIObservabilityApplications,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/observability/applications/{id}",
		h.miniAIObservabilityApplication,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/observability/services",
		h.miniAIObservabilityServices,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/observability/events",
		h.miniAIObservabilityEvents,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/deployments",
		h.miniAIDeployments,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/deployments/{app}",
		h.miniAIDeployment,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/deployments/{app}/history",
		h.miniAIDeploymentHistory,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/databases",
		h.miniAIDatabases,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/databases/{id}",
		h.miniAIDatabase,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/databases/{id}/backups",
		h.miniAIDatabaseBackups,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/activity",
		h.miniAIActivity,
	)
	h.mux.HandleFunc(
		"GET /internal/miniai/v1/recovery",
		h.miniAIRecovery,
	)
}

func (h *Handler) miniAINow() time.Time {
	if h.now == nil {
		return time.Now().UTC()
	}
	return h.now().UTC()
}

func miniAIContext(
	request *http.Request,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(request.Context(), miniAIRequestTimeout)
}

func writeMiniAIError(
	response http.ResponseWriter,
	status int,
	code string,
) {
	writeJSON(response, status, struct {
		Error string `json:"error"`
	}{Error: code})
}

func (h *Handler) miniAIWindow(
	response http.ResponseWriter,
	request *http.Request,
) (observability.Range, intelligence.Window, bool) {
	if h.observability == nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"observability_unavailable",
		)
		return observability.Range{}, intelligence.Window{}, false
	}
	window, metadata, err := intelligence.ResolveWindow(
		request.URL.Query(),
		h.miniAINow(),
	)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusBadRequest,
			"invalid_window",
		)
		return observability.Range{}, intelligence.Window{}, false
	}
	return window, metadata, true
}

func queryLimit(
	values url.Values,
	defaultValue int,
	maximum int,
) (int, error) {
	raw, present := values["limit"]
	if !present {
		return defaultValue, nil
	}
	if len(raw) != 1 {
		return 0, errors.New("invalid limit")
	}
	value, err := strconv.Atoi(raw[0])
	if err != nil || value < 1 || value > maximum {
		return 0, errors.New("invalid limit")
	}
	return value, nil
}

func (h *Handler) miniAIOverview(
	response http.ResponseWriter,
	request *http.Request,
) {
	ctx, cancel := miniAIContext(request)
	defer cancel()
	now := h.miniAINow()

	type systemResult struct {
		data intelligence.SystemSnapshot
		err  error
	}
	type recoveryResult struct {
		data intelligence.Recovery
	}
	type deploymentResult struct {
		data intelligence.DeploymentCollection
		err  error
	}
	type databaseResult struct {
		data intelligence.DatabaseCollection
		err  error
	}
	type observabilityResult struct {
		data intelligence.ObservabilityCurrentState
		err  error
	}

	systemResults := make(chan systemResult, 1)
	recoveryResults := make(chan recoveryResult, 1)
	deploymentResults := make(chan deploymentResult, 1)
	databaseResults := make(chan databaseResult, 1)
	observabilityResults := make(chan observabilityResult, 1)

	go func() {
		collect := h.collectSystem
		if collect == nil {
			collect = reactorsystem.Collect
		}
		metrics, err := collect()
		systemResults <- systemResult{
			data: intelligence.ProjectSystem(metrics),
			err:  err,
		}
	}()
	go func() {
		recoveryResults <- recoveryResult{
			data: h.collectMiniAIRecovery(ctx),
		}
	}()
	go func() {
		if h.miniAIDeploy == nil {
			deploymentResults <- deploymentResult{
				err: errors.New("deployment source unavailable"),
			}
			return
		}
		items, err := h.miniAIDeploy.DeploymentMetadata(ctx)
		deploymentResults <- deploymentResult{
			data: intelligence.ProjectDeployments(items),
			err:  err,
		}
	}()
	go func() {
		if h.miniAIBase == nil {
			databaseResults <- databaseResult{
				err: errors.New("database source unavailable"),
			}
			return
		}
		snapshot, err := h.miniAIBase.ValidatedDatabases(ctx)
		if err == nil {
			var projected intelligence.DatabaseCollection
			projected, err = intelligence.ProjectDatabases(snapshot)
			databaseResults <- databaseResult{data: projected, err: err}
			return
		}
		databaseResults <- databaseResult{err: err}
	}()
	go func() {
		if h.observability == nil {
			observabilityResults <- observabilityResult{
				err: errors.New("observability unavailable"),
			}
			return
		}
		window, _, err := intelligence.ResolveWindow(
			url.Values{"range": []string{"15m"}},
			now,
		)
		if err != nil {
			observabilityResults <- observabilityResult{err: err}
			return
		}
		applications, appErr := h.observability.QueryApplications(ctx, window)
		if appErr != nil {
			observabilityResults <- observabilityResult{err: appErr}
			return
		}
		services, serviceErr := h.observability.QueryServices(ctx, window)
		if serviceErr != nil {
			observabilityResults <- observabilityResult{err: serviceErr}
			return
		}
		if len(applications) > 500 || !boundedServiceSeries(services) {
			observabilityResults <- observabilityResult{
				err: errors.New("observability result exceeds bounds"),
			}
			return
		}
		observabilityResults <- observabilityResult{
			data: intelligence.ObservabilityCurrentState{
				Applications: intelligence.ProjectApplicationSummaries(
					applications,
				),
				Services: intelligence.ProjectServices(services),
			},
		}
	}()

	result := intelligence.Overview{CollectedAt: now}
	system := <-systemResults
	if system.err == nil {
		result.System = availableSection(system.data)
	} else {
		result.System = unavailableSection[intelligence.SystemSnapshot](
			"system_metrics_unavailable",
		)
	}
	recovery := <-recoveryResults
	result.Recovery = availableSection(recovery.data)
	deployments := <-deploymentResults
	if deployments.err == nil {
		result.Deployments = availableSection(deployments.data)
	} else {
		result.Deployments = unavailableSection[intelligence.DeploymentCollection]("deployment_source_unavailable")
	}
	databases := <-databaseResults
	if databases.err == nil {
		result.Databases = availableSection(databases.data)
	} else {
		result.Databases = unavailableSection[intelligence.DatabaseCollection]("database_source_unavailable")
	}
	current := <-observabilityResults
	if current.err == nil {
		result.Observability = availableSection(current.data)
	} else {
		result.Observability = unavailableSection[intelligence.ObservabilityCurrentState]("observability_unavailable")
	}
	writeJSON(response, http.StatusOK, result)
}

func availableSection[T any](data T) intelligence.Section[T] {
	return intelligence.Section[T]{Available: true, Data: &data}
}

func unavailableSection[T any](code string) intelligence.Section[T] {
	return intelligence.Section[T]{Available: false, Error: code}
}

func (h *Handler) miniAIObservabilityHost(
	response http.ResponseWriter,
	request *http.Request,
) {
	window, metadata, ok := h.miniAIWindow(response, request)
	if !ok {
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	points, err := h.observability.QueryHost(ctx, window)
	if err != nil || len(points) > intelligence.MaxMetricPoints {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"observability_unavailable",
		)
		return
	}
	writeJSON(response, http.StatusOK, intelligence.HostResponse{
		Window: metadata,
		Points: intelligence.ProjectHost(points),
	})
}

func (h *Handler) miniAIObservabilityTemperature(
	response http.ResponseWriter,
	request *http.Request,
) {
	window, metadata, ok := h.miniAIWindow(response, request)
	if !ok {
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	points, err := h.observability.QueryTemperature(ctx, window)
	if err != nil || len(points) > intelligence.MaxMetricPoints {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"observability_unavailable",
		)
		return
	}
	writeJSON(response, http.StatusOK, intelligence.TemperatureResponse{
		Window: metadata,
		Points: intelligence.ProjectTemperatures(points),
	})
}

func (h *Handler) miniAIObservabilityApplications(
	response http.ResponseWriter,
	request *http.Request,
) {
	window, metadata, ok := h.miniAIWindow(response, request)
	if !ok {
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	applications, err := h.observability.QueryApplications(ctx, window)
	if err != nil || len(applications) > 500 {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"observability_unavailable",
		)
		return
	}
	writeJSON(response, http.StatusOK, intelligence.ApplicationsResponse{
		Window:       metadata,
		Applications: intelligence.ProjectApplicationSummaries(applications),
	})
}

func (h *Handler) miniAIObservabilityApplication(
	response http.ResponseWriter,
	request *http.Request,
) {
	id := request.PathValue("id")
	if !observabilityResourceIDPattern.MatchString(id) {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_resource_id")
		return
	}
	window, metadata, ok := h.miniAIWindow(response, request)
	if !ok {
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	points, err := h.observability.QueryApplication(ctx, id, window)
	if err != nil || len(points) > intelligence.MaxMetricPoints {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"observability_unavailable",
		)
		return
	}
	writeJSON(response, http.StatusOK, intelligence.ApplicationResponse{
		Window: metadata,
		ID:     id,
		Points: intelligence.ProjectApplicationPoints(points),
	})
}

func (h *Handler) miniAIObservabilityServices(
	response http.ResponseWriter,
	request *http.Request,
) {
	window, metadata, ok := h.miniAIWindow(response, request)
	if !ok {
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	services, err := h.observability.QueryServices(ctx, window)
	if err != nil || !boundedServiceSeries(services) {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"observability_unavailable",
		)
		return
	}
	writeJSON(response, http.StatusOK, intelligence.ServicesResponse{
		Window:   metadata,
		Services: intelligence.ProjectServices(services),
	})
}

func boundedServiceSeries(series []observability.ServiceSeries) bool {
	if len(series) > 500 {
		return false
	}
	for _, service := range series {
		if len(service.Points) > intelligence.MaxMetricPoints {
			return false
		}
	}
	return true
}

func (h *Handler) miniAIObservabilityEvents(
	response http.ResponseWriter,
	request *http.Request,
) {
	window, metadata, ok := h.miniAIWindow(response, request)
	if !ok {
		return
	}
	limit, err := queryLimit(
		request.URL.Query(),
		intelligence.DefaultEventResults,
		intelligence.MaxEventResults,
	)
	if err != nil {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_limit")
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	events, err := h.observability.QueryEvents(
		ctx,
		window.From,
		window.To,
		limit,
	)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"observability_unavailable",
		)
		return
	}
	if len(events) > limit {
		events = events[:limit]
	}
	writeJSON(response, http.StatusOK, intelligence.EventsResponse{
		Window: metadata,
		Limit:  limit,
		Events: intelligence.ProjectEvents(events),
	})
}

func (h *Handler) deploymentCollection(
	ctx context.Context,
) (intelligence.DeploymentCollection, error) {
	if h.miniAIDeploy == nil {
		return intelligence.DeploymentCollection{},
			errors.New("deployment source unavailable")
	}
	items, err := h.miniAIDeploy.DeploymentMetadata(ctx)
	if err != nil {
		return intelligence.DeploymentCollection{}, err
	}
	return intelligence.ProjectDeployments(items), nil
}

func (h *Handler) miniAIDeployments(
	response http.ResponseWriter,
	request *http.Request,
) {
	ctx, cancel := miniAIContext(request)
	defer cancel()
	collection, err := h.deploymentCollection(ctx)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"deployment_source_unavailable",
		)
		return
	}
	writeJSON(response, http.StatusOK, collection)
}

func (h *Handler) miniAIDeployment(
	response http.ResponseWriter,
	request *http.Request,
) {
	app := request.PathValue("app")
	if !minideploy.ValidApplicationID(app) {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_app")
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	collection, err := h.deploymentCollection(ctx)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"deployment_source_unavailable",
		)
		return
	}
	deployment, ok := intelligence.FindDeployment(collection, app)
	if !ok {
		writeMiniAIError(
			response,
			http.StatusNotFound,
			"deployment_not_found",
		)
		return
	}
	writeJSON(response, http.StatusOK, deployment)
}

func (h *Handler) miniAIDeploymentHistory(
	response http.ResponseWriter,
	request *http.Request,
) {
	app := request.PathValue("app")
	if !minideploy.ValidApplicationID(app) {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_app")
		return
	}
	if h.miniAIDeploy == nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"deployment_source_unavailable",
		)
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	history, err := h.miniAIDeploy.DeploymentHistory(ctx, app)
	if errors.Is(err, minideploy.ErrDeploymentNotFound) {
		writeMiniAIError(
			response,
			http.StatusNotFound,
			"deployment_not_found",
		)
		return
	}
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"deployment_history_unavailable",
		)
		return
	}
	writeJSON(
		response,
		http.StatusOK,
		intelligence.ProjectDeploymentHistory(history),
	)
}

func (h *Handler) databaseCollection(
	ctx context.Context,
) (intelligence.DatabaseCollection, error) {
	if h.miniAIBase == nil {
		return intelligence.DatabaseCollection{},
			errors.New("database source unavailable")
	}
	snapshot, err := h.miniAIBase.ValidatedDatabases(ctx)
	if err != nil {
		return intelligence.DatabaseCollection{}, err
	}
	return intelligence.ProjectDatabases(snapshot)
}

func (h *Handler) miniAIDatabases(
	response http.ResponseWriter,
	request *http.Request,
) {
	ctx, cancel := miniAIContext(request)
	defer cancel()
	collection, err := h.databaseCollection(ctx)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"database_source_unavailable",
		)
		return
	}
	writeJSON(response, http.StatusOK, collection)
}

func (h *Handler) miniAIDatabase(
	response http.ResponseWriter,
	request *http.Request,
) {
	id := request.PathValue("id")
	if !minibase.ValidDatabaseID(id) {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_database_id")
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	collection, err := h.databaseCollection(ctx)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"database_source_unavailable",
		)
		return
	}
	database, ok := intelligence.FindDatabase(collection, id)
	if !ok {
		writeMiniAIError(
			response,
			http.StatusNotFound,
			"database_not_found",
		)
		return
	}
	writeJSON(response, http.StatusOK, database)
}

func (h *Handler) miniAIDatabaseBackups(
	response http.ResponseWriter,
	request *http.Request,
) {
	id := request.PathValue("id")
	if !minibase.ValidDatabaseID(id) {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_database_id")
		return
	}
	limit, err := queryLimit(
		request.URL.Query(),
		intelligence.DefaultBackupResults,
		intelligence.MaxBackupResults,
	)
	if err != nil {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_limit")
		return
	}
	if h.miniAIBase == nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"database_source_unavailable",
		)
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	collection, err := h.databaseCollection(ctx)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"database_source_unavailable",
		)
		return
	}
	if _, ok := intelligence.FindDatabase(collection, id); !ok {
		writeMiniAIError(
			response,
			http.StatusNotFound,
			"database_not_found",
		)
		return
	}
	backups, err := h.miniAIBase.DatabaseBackups(ctx, id)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"backup_source_unavailable",
		)
		return
	}
	writeJSON(
		response,
		http.StatusOK,
		intelligence.ProjectBackups(id, backups, limit),
	)
}

func (h *Handler) collectMiniAIRecovery(
	ctx context.Context,
) intelligence.Recovery {
	inspectHardware := h.inspectHardwareWatchdogProtection
	if inspectHardware == nil {
		inspectHardware = reactorsystem.InspectHardwareWatchdogProtection
	}
	hardware, hardwareErr := inspectHardware(ctx)
	if hardwareErr != nil ||
		!reactorsystem.ValidRecoveryProtectionState(hardware.State) {

		hardware = reactorsystem.HardwareWatchdogProtectionState{
			State: reactorsystem.RecoveryProtectionUnavailable,
		}
	}
	inspectRTC := h.inspectRTCRecoveryProtection
	if inspectRTC == nil {
		inspectRTC = reactorsystem.InspectRTCRecoveryProtection
	}
	rtc, rtcErr := inspectRTC(ctx)
	if rtcErr != nil ||
		!reactorsystem.ValidRecoveryProtectionState(rtc.State) {

		rtc = reactorsystem.RTCRecoveryProtectionState{
			State: reactorsystem.RecoveryProtectionUnavailable,
		}
	}
	protection := reactorsystem.RecoveryProtectionState{
		State: reactorsystem.AggregateRecoveryProtectionState(
			hardware.State,
			rtc.State,
		),
		HardwareWatchdog: hardware,
		RTC:              rtc,
	}
	result := intelligence.Recovery{
		Protection:      intelligence.ProjectProtection(protection),
		RecentIncidents: []intelligence.RecoveryIncident{},
	}
	if h.observability == nil {
		return result
	}
	incidents, err := h.observability.ListRecoveryIncidents(
		ctx,
		miniAIRecoveryLimit,
	)
	if err != nil {
		return result
	}
	sortRecoveryIncidents(incidents)
	result.HistoryAvailable = true
	for _, incident := range incidents {
		result.RecentIncidents = append(
			result.RecentIncidents,
			intelligence.ProjectRecoveryIncident(incident),
		)
	}
	if len(result.RecentIncidents) > 0 {
		latest := result.RecentIncidents[0]
		result.LastIncident = &latest
	} else {
		latest, latestErr := h.observability.LatestRecoveryIncident(ctx)
		if latestErr != nil {
			result.HistoryAvailable = false
		} else if latest != nil {
			projected := intelligence.ProjectRecoveryIncident(*latest)
			result.LastIncident = &projected
		}
	}
	return result
}

func (h *Handler) miniAIRecovery(
	response http.ResponseWriter,
	request *http.Request,
) {
	ctx, cancel := miniAIContext(request)
	defer cancel()
	writeJSON(response, http.StatusOK, h.collectMiniAIRecovery(ctx))
}

func (h *Handler) miniAIActivity(
	response http.ResponseWriter,
	request *http.Request,
) {
	limit, err := queryLimit(
		request.URL.Query(),
		intelligence.DefaultActivityResults,
		intelligence.MaxActivityResults,
	)
	if err != nil {
		writeMiniAIError(response, http.StatusBadRequest, "invalid_limit")
		return
	}
	if h.activity == nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"activity_unavailable",
		)
		return
	}
	ctx, cancel := miniAIContext(request)
	defer cancel()
	legacy, err := h.activity.ListActivity(ctx, limit)
	if err != nil {
		writeMiniAIError(
			response,
			http.StatusServiceUnavailable,
			"activity_unavailable",
		)
		return
	}
	recoveries := []observability.RecoveryIncident{}
	if h.observability != nil {
		if items, recoveryErr := h.observability.ListRecoveryIncidents(
			ctx,
			limit,
		); recoveryErr == nil {

			recoveries = items
		}
	}
	writeJSON(
		response,
		http.StatusOK,
		intelligence.ProjectActivity(legacy, recoveries, limit),
	)
}

func sortRecoveryIncidents(
	incidents []observability.RecoveryIncident,
) {
	sort.SliceStable(incidents, func(i, j int) bool {
		return incidents[i].RecoveredAt.After(incidents[j].RecoveredAt)
	})
}
