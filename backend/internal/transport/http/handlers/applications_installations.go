package httphandlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	svc "github.com/futrx-com/remote.futrx.com/internal/service/applications"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
)

// Installation state is authorized by its requested scope before dispatch:
// global routes require an admin; project routes require project membership.
func (h *ApplicationsHandler) handleInstallations(w http.ResponseWriter, r *http.Request, scope svc.Scope, project, resource string) {
	w.Header().Set("Cache-Control", "no-store")
	if resource == "installations" && r.Method == http.MethodGet {
		runs, err := h.apps.Installations(r.Context(), scope, project)
		if err != nil {
			sendAppError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, runs)
		return
	}
	if resource == "installations" && r.Method == http.MethodPost {
		var body installBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		run, err := h.apps.StartInstall(r.Context(), svc.InstallRequest{
			ApplicationID: strings.TrimSpace(body.ApplicationID), Scope: scope, ProjectID: project,
			Name: body.Name, Env: body.Env, ExternalPort: body.ExternalPort, BindAddress: body.BindAddress,
		})
		if err != nil {
			sendAppError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusAccepted, run)
		return
	}
	app := strings.TrimPrefix(resource, "installations/")
	if resource == "installations" || app == "" || strings.Contains(app, "/") {
		httptransport.SendErr(w, http.StatusNotFound, "installation not found")
		return
	}
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	run, found, err := h.apps.Installation(r.Context(), scope, project, app)
	if err != nil {
		sendAppError(w, err)
		return
	}
	if !found {
		httptransport.SendJSON(w, http.StatusOK, nil)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, run)
}
