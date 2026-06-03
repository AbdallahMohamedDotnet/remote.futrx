package httphandlers

// login_sessions_handler.go: HTTP API for starting and stopping the
// Chromium-driven login session capture flow. Streaming + capture endpoints
// live alongside this in later milestones (WebSocket socket, capture POST).
//
// This handler does NOT register its own mux routes — the ProjectHandler
// already owns the `/api/projects/` prefix and dispatches into us via
// HandleSubresource so we don't clash on the shared prefix.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	loginsessions "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/loginsessions"
	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

// LoginSessionProjectGetter is the subset of project.Service the login
// handler needs. Kept narrow so tests can stub it without spinning the full
// service set.
type LoginSessionProjectGetter interface {
	Get(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	Start(ctx context.Context, id serviceproject.ID) (serviceproject.Meta, error)
	SetSecret(ctx context.Context, id serviceproject.ID, key, value string) (serviceproject.Secret, error)
}

type LoginSessionHandler struct {
	manager  *loginsessions.Manager
	projects LoginSessionProjectGetter
	auth     *serviceauth.Service
}

func NewLoginSessionHandler(
	manager *loginsessions.Manager,
	projects LoginSessionProjectGetter,
	auth *serviceauth.Service,
) *LoginSessionHandler {
	return &LoginSessionHandler{manager: manager, projects: projects, auth: auth}
}

// HandleSubresource is called by ProjectHandler after caller auth + project
// access have already been gated. parts[0]==<projectID>, parts[1]=="login-sessions",
// parts[2:] is the rest of the path.
func (h *LoginSessionHandler) HandleSubresource(w http.ResponseWriter, r *http.Request, id serviceproject.ID, parts []string) {
	if h == nil {
		httptransport.SendErr(w, http.StatusNotFound, "login-sessions disabled")
		return
	}
	if len(parts) == 2 {
		h.handleCollection(w, r, id)
		return
	}

	sid := parts[2]
	if sid == "" {
		httptransport.SendErr(w, http.StatusBadRequest, "missing session id")
		return
	}
	if len(parts) == 3 {
		h.handleResource(w, r, id, sid)
		return
	}
	if len(parts) >= 4 {
		switch parts[3] {
		case "capture":
			h.handleCapture(w, r, id, sid)
			return
		}
	}
	httptransport.SendErr(w, http.StatusNotFound, "unknown login-session action")
}

type startBody struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type startResponse struct {
	ID         string `json:"id"`
	WSPath     string `json:"wsPath"`
	URL        string `json:"url"`
	Name       string `json:"name"`
	SecretName string `json:"secretName"`
	ExpiresAt  int64  `json:"expiresAt"`
}

func (h *LoginSessionHandler) handleCollection(w http.ResponseWriter, r *http.Request, id serviceproject.ID) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body startBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	body.Name = strings.TrimSpace(body.Name)
	if body.URL == "" {
		httptransport.SendErr(w, http.StatusBadRequest, "url is required")
		return
	}
	if body.Name == "" {
		httptransport.SendErr(w, http.StatusBadRequest, "name is required")
		return
	}

	// Pre-validate secret name so we fail fast (before spinning up Chromium)
	// if the user passes something that won't sanitize cleanly.
	if _, _, err := loginsessions.SanitizeSecretName(body.Name); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}

	project, err := h.projects.Get(r.Context(), id)
	if err != nil {
		httptransport.SendErr(w, http.StatusNotFound, "project not found")
		return
	}
	if project.ContainerName == "" {
		httptransport.SendErr(w, http.StatusBadRequest, "project has no container")
		return
	}
	// Container must be running for lxc exec to work — start it lazily.
	if project.Status != serviceproject.StatusRunning {
		started, err := h.projects.Start(r.Context(), id)
		if err != nil {
			httptransport.SendErr(w, http.StatusInternalServerError, "start container: "+err.Error())
			return
		}
		project = started
	}

	sess, err := h.manager.Start(r.Context(), loginsessions.ProjectMeta{
		ID:            string(project.ID),
		Slug:          project.Slug,
		ContainerName: project.ContainerName,
	}, body.URL, body.Name)
	if err != nil {
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	httptransport.SendJSON(w, http.StatusCreated, startResponse{
		ID:         sess.ID,
		WSPath:     "/ws/login-session/" + sess.ID,
		URL:        sess.URL,
		Name:       sess.Name,
		SecretName: sess.SecretName,
		ExpiresAt:  sess.ExpiresAt.Unix(),
	})
}

func (h *LoginSessionHandler) handleResource(w http.ResponseWriter, r *http.Request, id serviceproject.ID, sid string) {
	sess, ok := h.manager.Get(sid)
	if !ok {
		httptransport.SendErr(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.ProjectID != string(id) {
		httptransport.SendErr(w, http.StatusNotFound, "session not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		httptransport.SendJSON(w, http.StatusOK, sess)
	case http.MethodDelete:
		if err := h.manager.Stop(r.Context(), sid); err != nil {
			httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		httptransport.SendJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type captureResponse struct {
	SecretName  string `json:"secretName"`
	SizeBytes   int    `json:"sizeBytes"`
	CookieCount int    `json:"cookieCount"`
	OriginCount int    `json:"originCount"`
}

// handleCapture snapshots cookies + per-origin localStorage from the
// running Chromium session, writes the result as the project secret
// STORAGE_STATE_<NAME>, and stops the session.
func (h *LoginSessionHandler) handleCapture(w http.ResponseWriter, r *http.Request, id serviceproject.ID, sid string) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sess, ok := h.manager.Get(sid)
	if !ok {
		httptransport.SendErr(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.ProjectID != string(id) {
		httptransport.SendErr(w, http.StatusNotFound, "session not found")
		return
	}

	result, err := h.manager.Capture(r.Context(), sid)
	if err != nil {
		httptransport.SendErr(w, http.StatusInternalServerError, "capture: "+err.Error())
		return
	}

	if _, err := h.projects.SetSecret(r.Context(), id, result.SecretName, result.JSON); err != nil {
		httptransport.SendErr(w, http.StatusInternalServerError, "save secret: "+err.Error())
		return
	}

	// Best-effort stop; don't fail the request if cleanup fails (the
	// secret is already saved, the session will expire anyway).
	if stopErr := h.manager.Stop(r.Context(), sid); stopErr != nil {
		// We log via the response side-channel — there's no logger here
		// and a non-fatal stop failure shouldn't 500 the request.
		_ = stopErr
	}

	httptransport.SendJSON(w, http.StatusOK, captureResponse{
		SecretName:  result.SecretName,
		SizeBytes:   result.SizeBytes,
		CookieCount: result.CookieCount,
		OriginCount: result.OriginCount,
	})
}
