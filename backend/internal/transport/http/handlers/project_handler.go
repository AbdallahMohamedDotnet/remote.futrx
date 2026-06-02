package httphandlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type ProjectHandler struct {
	projects *serviceproject.Service
}

func NewProjectHandler(projects *serviceproject.Service) *ProjectHandler {
	return &ProjectHandler{projects: projects}
}

func (h *ProjectHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/projects", h.HandleCollection)
	mux.HandleFunc("/api/projects/", h.HandleResource)
	mux.HandleFunc("/internal/tls-ask", h.HandleTLSAsk)
}

func (h *ProjectHandler) HandleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		metas, err := h.projects.List(r.Context())
		if err != nil {
			sendProjectError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, metas)

	case http.MethodPost:
		var body serviceproject.CreateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		m, err := h.projects.Create(r.Context(), body)
		if err != nil {
			sendProjectError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusCreated, m)

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *ProjectHandler) HandleResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.SplitN(rest, "/", 3)
	id := serviceproject.ID(parts[0])
	if id == "" {
		httptransport.SendErr(w, http.StatusBadRequest, "missing id")
		return
	}

	if len(parts) >= 2 && parts[1] == "secrets" {
		h.handleSecrets(w, r, id, parts)
		return
	}

	if len(parts) >= 2 && parts[1] != "" {
		switch parts[1] {
		case "start":
			if r.Method != http.MethodPost {
				httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			m, err := h.projects.Start(r.Context(), id)
			if err != nil {
				sendProjectError(w, err)
				return
			}
			httptransport.SendJSON(w, http.StatusOK, m)
		case "stop":
			if r.Method != http.MethodPost {
				httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			m, err := h.projects.Stop(r.Context(), id)
			if err != nil {
				sendProjectError(w, err)
				return
			}
			httptransport.SendJSON(w, http.StatusOK, m)
		case "container":
			if r.Method != http.MethodGet {
				httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			info, err := h.projects.InspectContainer(r.Context(), id)
			if err != nil {
				sendProjectError(w, err)
				return
			}
			httptransport.SendJSON(w, http.StatusOK, info)
		default:
			httptransport.SendErr(w, http.StatusNotFound, "unknown action")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		m, err := h.projects.Get(r.Context(), id)
		if err != nil {
			sendProjectError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, m)

	case http.MethodPatch:
		var body serviceproject.UpdateInput
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		m, err := h.projects.Update(r.Context(), id, body)
		if err != nil {
			sendProjectError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, m)

	case http.MethodDelete:
		if err := h.projects.Delete(r.Context(), id); err != nil {
			sendProjectError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

var projectHostPattern = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)--(\d{4,5})\.dev\.remote\.futrx\.dev$`)

func (h *ProjectHandler) HandleTLSAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	domain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
	if domain == "" {
		http.Error(w, "missing domain", http.StatusBadRequest)
		return
	}
	m := projectHostPattern.FindStringSubmatch(domain)
	if m == nil {
		http.Error(w, "host does not match <slug>--<port>.dev.remote.futrx.dev", http.StatusNotFound)
		return
	}
	slug := m[1]
	port, err := strconv.Atoi(m[2])
	if err != nil || port < 1024 || port > 65535 {
		http.Error(w, "port out of range", http.StatusNotFound)
		return
	}
	if _, err := h.projects.GetBySlug(r.Context(), slug); err != nil {
		http.Error(w, "no such project", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProjectHandler) handleSecrets(w http.ResponseWriter, r *http.Request, id serviceproject.ID, parts []string) {
	// /api/projects/{id}/secrets[/{key}]
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		secrets, err := h.projects.ListSecrets(r.Context(), id)
		if err != nil {
			sendProjectError(w, err)
			return
		}
		if secrets == nil {
			secrets = []serviceproject.Secret{}
		}
		httptransport.SendJSON(w, http.StatusOK, secrets)
		return
	}

	key := parts[2]
	if key == "" {
		httptransport.SendErr(w, http.StatusBadRequest, "missing secret key")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		s, err := h.projects.SetSecret(r.Context(), id, key, body.Value)
		if err != nil {
			sendProjectError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, s)
	case http.MethodDelete:
		if err := h.projects.DeleteSecret(r.Context(), id, key); err != nil {
			sendProjectError(w, err)
			return
		}
		httptransport.SendJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func sendProjectError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, serviceproject.ErrNameRequired),
		errors.Is(err, serviceproject.ErrInvalidID),
		errors.Is(err, serviceproject.ErrInvalidSecretKey):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, serviceproject.ErrSecretsUnavailable):
		httptransport.SendErr(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, serviceproject.ErrNotFound):
		httptransport.SendErr(w, http.StatusNotFound, "project not found")
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
