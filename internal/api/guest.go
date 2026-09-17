package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/conradevans/ReactorLab/internal/minibase"
	"github.com/conradevans/ReactorLab/internal/minideploy"
)

type guestDeploymentSource interface {
	GuestDeployments(context.Context) (minideploy.GuestDeployments, error)
}

type guestDatabaseSource interface {
	GuestDatabases(context.Context) (minibase.GuestDatabases, error)
}

type guestResourceSummary struct {
	Total  int `json:"total"`
	Shared int `json:"shared"`
	Hidden int `json:"hidden"`
}

type guestDeploymentItem struct {
	App    string `json:"app"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

type guestDatabaseItem struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
}

type guestDeploymentSection struct {
	Available bool                  `json:"available"`
	Summary   guestResourceSummary  `json:"summary"`
	Items     []guestDeploymentItem `json:"items"`
}

type guestDatabaseSection struct {
	Available bool                 `json:"available"`
	Summary   guestResourceSummary `json:"summary"`
	Items     []guestDatabaseItem  `json:"items"`
}

type guestResourcesResponse struct {
	Deployments guestDeploymentSection `json:"deployments"`
	Databases   guestDatabaseSection   `json:"databases"`
}

type guestDeploymentResult struct {
	snapshot minideploy.GuestDeployments
	err      error
}

type guestDatabaseResult struct {
	snapshot minibase.GuestDatabases
	err      error
}

func (h *Handler) guestResources(w http.ResponseWriter, r *http.Request) {
	deploymentResults := make(chan guestDeploymentResult, 1)
	databaseResults := make(chan guestDatabaseResult, 1)

	go func() {
		if h.guestMiniDeploy == nil {
			deploymentResults <- guestDeploymentResult{
				err: errors.New("MiniDeploy Guest source unavailable"),
			}
			return
		}
		snapshot, err := h.guestMiniDeploy.GuestDeployments(r.Context())
		deploymentResults <- guestDeploymentResult{snapshot: snapshot, err: err}
	}()

	go func() {
		if h.guestMiniBase == nil {
			databaseResults <- guestDatabaseResult{
				err: errors.New("MiniBase Guest source unavailable"),
			}
			return
		}
		snapshot, err := h.guestMiniBase.GuestDatabases(r.Context())
		databaseResults <- guestDatabaseResult{snapshot: snapshot, err: err}
	}()

	deploymentResult := <-deploymentResults
	databaseResult := <-databaseResults

	response := guestResourcesResponse{
		Deployments: guestDeploymentSection{
			Items: []guestDeploymentItem{},
		},
		Databases: guestDatabaseSection{
			Items: []guestDatabaseItem{},
		},
	}

	if deploymentResult.err == nil &&
		validDeploymentGuestSnapshot(deploymentResult.snapshot) {
		response.Deployments = projectGuestDeployments(deploymentResult.snapshot)
	}
	if databaseResult.err == nil &&
		validDatabaseGuestSnapshot(databaseResult.snapshot) {
		response.Databases = projectGuestDatabases(databaseResult.snapshot)
	}

	writeJSON(w, http.StatusOK, response)
}

func validDeploymentGuestSnapshot(snapshot minideploy.GuestDeployments) bool {
	return snapshot.Summary.Total >= 0 &&
		snapshot.Summary.Showing >= 0 &&
		snapshot.Summary.Hidden >= 0 &&
		snapshot.Summary.Total ==
			snapshot.Summary.Showing+snapshot.Summary.Hidden &&
		snapshot.Summary.Showing == len(snapshot.Deployments)
}

func validDatabaseGuestSnapshot(snapshot minibase.GuestDatabases) bool {
	return snapshot.Summary.Total >= 0 &&
		snapshot.Summary.Showing >= 0 &&
		snapshot.Summary.Hidden >= 0 &&
		snapshot.Summary.Total ==
			snapshot.Summary.Showing+snapshot.Summary.Hidden &&
		snapshot.Summary.Showing == len(snapshot.Databases)
}

func projectGuestDeployments(
	snapshot minideploy.GuestDeployments,
) guestDeploymentSection {
	items := make([]guestDeploymentItem, 0, len(snapshot.Deployments))
	for _, deployment := range snapshot.Deployments {
		items = append(items, guestDeploymentItem{
			App:    deployment.App,
			URL:    deployment.URL,
			Status: deployment.Status,
		})
	}

	return guestDeploymentSection{
		Available: true,
		Summary: guestResourceSummary{
			Total:  snapshot.Summary.Total,
			Shared: snapshot.Summary.Showing,
			Hidden: snapshot.Summary.Hidden,
		},
		Items: items,
	}
}

func projectGuestDatabases(
	snapshot minibase.GuestDatabases,
) guestDatabaseSection {
	items := make([]guestDatabaseItem, 0, len(snapshot.Databases))
	for _, database := range snapshot.Databases {
		items = append(items, guestDatabaseItem{
			ID:          database.ID,
			DisplayName: database.DisplayName,
			Status:      database.Status,
		})
	}

	return guestDatabaseSection{
		Available: true,
		Summary: guestResourceSummary{
			Total:  snapshot.Summary.Total,
			Shared: snapshot.Summary.Showing,
			Hidden: snapshot.Summary.Hidden,
		},
		Items: items,
	}
}
