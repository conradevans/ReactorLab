package api

import (
	"context"
	"time"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
)

const (
	linkStateLinked      = "linked"
	linkStateDetached    = "detached"
	linkStateUnavailable = "unavailable"
	linkStateUnresolved  = "unresolved"
	linkStateConflict    = "conflict"
)

type databaseLinkResponse struct {
	State       string `json:"state"`
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Status      string `json:"status,omitempty"`
}

type deploymentLinkResponse struct {
	State    string `json:"state"`
	App      string `json:"app,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Status   string `json:"status,omitempty"`
}

type deploymentResponse struct {
	App        string                        `json:"app"`
	Strategy   string                        `json:"strategy"`
	Status     string                        `json:"status"`
	Containers []minideploy.ContainerMetrics `json:"containers"`
	Database   databaseLinkResponse          `json:"database"`
}

type deploymentsResponse struct {
	Deployments []deploymentResponse `json:"deployments"`
	CollectedAt time.Time            `json:"collectedAt"`
}

type databaseResponse struct {
	ID                string                 `json:"id"`
	DisplayName       string                 `json:"displayName"`
	Status            string                 `json:"status"`
	SizeBytes         int64                  `json:"sizeBytes"`
	Connections       int64                  `json:"connections"`
	ActiveConnections int64                  `json:"activeConnections"`
	IdleConnections   int64                  `json:"idleConnections"`
	Transactions      minibase.Transactions  `json:"transactions"`
	Cache             minibase.CacheMetrics  `json:"cache"`
	Rows              minibase.RowMetrics    `json:"rows"`
	BackupCount       int                    `json:"backupCount"`
	BackupBytes       int64                  `json:"backupBytes"`
	LatestBackupAt    *time.Time             `json:"latestBackupAt,omitempty"`
	BackupAgeSeconds  *float64               `json:"backupAgeSeconds,omitempty"`
	Deployment        deploymentLinkResponse `json:"deployment"`
}

type databasesResponse struct {
	Databases   []databaseResponse       `json:"databases"`
	Postgres    minibase.PostgresMetrics `json:"postgres"`
	CollectedAt time.Time                `json:"collectedAt"`
}

func databaseLinkForDeployment(
	app string,
	snapshot minibase.Snapshot,
) databaseLinkResponse {
	var match *minibase.Database

	for index := range snapshot.Databases {
		database := &snapshot.Databases[index]
		for _, attachment := range database.Attachments {
			if attachment.ConsumerType != "minideploy" ||
				attachment.BindingName != "primary" ||
				attachment.ConsumerRef != app {
				continue
			}

			if match != nil && match.ID != database.ID {
				return databaseLinkResponse{State: linkStateConflict}
			}
			match = database
		}
	}

	if match == nil {
		return databaseLinkResponse{State: linkStateDetached}
	}

	return databaseLinkResponse{
		State:       linkStateLinked,
		ID:          match.ID,
		DisplayName: match.DisplayName,
		Status:      match.Status,
	}
}

func deploymentLinkForDatabase(
	database minibase.Database,
	snapshot minideploy.Snapshot,
) deploymentLinkResponse {
	consumerRefs := make(map[string]struct{})

	for _, attachment := range database.Attachments {
		if attachment.ConsumerType != "minideploy" ||
			attachment.BindingName != "primary" {
			continue
		}
		consumerRefs[attachment.ConsumerRef] = struct{}{}
	}

	if len(consumerRefs) == 0 {
		return deploymentLinkResponse{State: linkStateDetached}
	}
	if len(consumerRefs) != 1 {
		return deploymentLinkResponse{State: linkStateConflict}
	}

	var app string
	for consumerRef := range consumerRefs {
		app = consumerRef
	}

	deployment, ok := minideploy.FindDeployment(snapshot, app)
	if !ok {
		return deploymentLinkResponse{
			State: linkStateUnresolved,
			App:   app,
		}
	}

	return deploymentLinkResponse{
		State:    linkStateLinked,
		App:      deployment.App,
		Strategy: deployment.Strategy,
		Status:   deployment.Status,
	}
}

func unavailableDatabaseLink() databaseLinkResponse {
	return databaseLinkResponse{State: linkStateUnavailable}
}

func unavailableDeploymentLink() deploymentLinkResponse {
	return deploymentLinkResponse{State: linkStateUnavailable}
}

func projectDeployment(
	deployment minideploy.Deployment,
	link databaseLinkResponse,
) deploymentResponse {
	containers := deployment.Containers
	if containers == nil {
		containers = []minideploy.ContainerMetrics{}
	}

	return deploymentResponse{
		App:        deployment.App,
		Strategy:   deployment.Strategy,
		Status:     deployment.Status,
		Containers: containers,
		Database:   link,
	}
}

func projectDatabase(
	database minibase.Database,
	link deploymentLinkResponse,
) databaseResponse {
	return databaseResponse{
		ID:                database.ID,
		DisplayName:       database.DisplayName,
		Status:            database.Status,
		SizeBytes:         database.SizeBytes,
		Connections:       database.Connections,
		ActiveConnections: database.ActiveConnections,
		IdleConnections:   database.IdleConnections,
		Transactions:      database.Transactions,
		Cache:             database.Cache,
		Rows:              database.Rows,
		BackupCount:       database.BackupCount,
		BackupBytes:       database.BackupBytes,
		LatestBackupAt:    database.LatestBackupAt,
		BackupAgeSeconds:  database.BackupAgeSeconds,
		Deployment:        link,
	}
}

func (h *Handler) databaseSnapshotForLinks(
	ctx context.Context,
) (minibase.Snapshot, bool) {
	if h.miniBase == nil {
		return minibase.Snapshot{}, false
	}
	snapshot, err := h.miniBase.Databases(ctx)
	if err != nil {
		return minibase.Snapshot{}, false
	}
	return snapshot, true
}

func (h *Handler) deploymentSnapshotForLinks(
	ctx context.Context,
) (minideploy.Snapshot, bool) {
	if h.miniDeploy == nil {
		return minideploy.Snapshot{}, false
	}
	snapshot, err := h.miniDeploy.Deployments(ctx)
	if err != nil {
		return minideploy.Snapshot{}, false
	}
	return snapshot, true
}

type databaseSnapshotResult struct {
	snapshot  minibase.Snapshot
	available bool
}

type deploymentSnapshotResult struct {
	snapshot  minideploy.Snapshot
	available bool
}

func (h *Handler) databaseSnapshotForLinksAsync(
	ctx context.Context,
) <-chan databaseSnapshotResult {
	results := make(chan databaseSnapshotResult, 1)
	go func() {
		snapshot, available := h.databaseSnapshotForLinks(ctx)
		results <- databaseSnapshotResult{
			snapshot:  snapshot,
			available: available,
		}
	}()
	return results
}

func (h *Handler) deploymentSnapshotForLinksAsync(
	ctx context.Context,
) <-chan deploymentSnapshotResult {
	results := make(chan deploymentSnapshotResult, 1)
	go func() {
		snapshot, available := h.deploymentSnapshotForLinks(ctx)
		results <- deploymentSnapshotResult{
			snapshot:  snapshot,
			available: available,
		}
	}()
	return results
}
