package api

import (
	"net/http"
	"strings"
)

// NewPublicHandler exposes only ReactorLab's public guest surface.
// Administrator routes and APIs remain available only on the private listener.
func NewPublicHandler(frontendDir string) http.Handler {
	clients := newProductionServiceClients()
	return newPublicHandler(
		frontendDir,
		clients.guestMiniDeploy,
		clients.miniBase,
	)
}

func newPublicHandler(
	frontendDir string,
	miniDeploy guestDeploymentSource,
	miniBase guestDatabaseSource,
) http.Handler {
	h := &Handler{
		frontendDir:     frontendDir,
		guestMiniDeploy: miniDeploy,
		guestMiniBase:   miniBase,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		switch {
		case r.URL.Path == "/health":
			h.health(w, r)
		case r.URL.Path == "/api/v1/guest/status":
			h.guestStatus(w, r)
		case r.URL.Path == "/api/v1/guest/resources":
			h.guestResources(w, r)
		case r.URL.Path == "/",
			r.URL.Path == "/guest",
			r.URL.Path == "/guest/",
			r.URL.Path == "/favicon.svg",
			strings.HasPrefix(r.URL.Path, "/assets/"):
			h.frontend(w, r)
		default:
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusNotFound, map[string]any{
					"error": "not_found",
				})
				return
			}
			http.NotFound(w, r)
		}
	})
}
