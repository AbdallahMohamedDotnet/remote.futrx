package httphandlers

import (
	"errors"
	"net/http"
	"strings"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	serviceskills "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/skills"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type SkillHandler struct {
	skills   *serviceskills.Service
	projects *serviceproject.Service
	auth     *serviceauth.Service
}

func NewSkillHandler(
	skills *serviceskills.Service,
	projects *serviceproject.Service,
	auth *serviceauth.Service,
) *SkillHandler {
	return &SkillHandler{skills: skills, projects: projects, auth: auth}
}

func (h *SkillHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/skills", h.HandleCollection)
}

func (h *SkillHandler) HandleCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.skills == nil {
		httptransport.SendErr(w, http.StatusServiceUnavailable, "skills unavailable")
		return
	}

	provider := serviceskills.Provider(strings.TrimSpace(r.URL.Query().Get("provider")))
	if provider == "" {
		provider = serviceskills.ProviderCodex
	}

	workspacePath := ""
	if projectID := serviceproject.ID(strings.TrimSpace(r.URL.Query().Get("projectId"))); projectID != "" {
		if h.projects == nil || h.auth == nil {
			httptransport.SendErr(w, http.StatusServiceUnavailable, "project lookup unavailable")
			return
		}
		meta, err := h.projects.Get(r.Context(), projectID)
		if err != nil {
			if errors.Is(err, serviceproject.ErrNotFound) {
				httptransport.SendErr(w, http.StatusNotFound, "project not found")
				return
			}
			httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		email, err := callerEmailFromRequest(r, h.auth)
		if err != nil || email == "" {
			httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if admin, _ := h.auth.IsAdmin(r.Context(), email); !admin {
			if ok, _ := h.projects.HasAccess(r.Context(), meta.ID, email); !ok {
				httptransport.SendErr(w, http.StatusForbidden, "project access denied")
				return
			}
		}
		workspacePath = meta.Cwd
	}

	items, err := h.skills.List(r.Context(), provider, workspacePath)
	if err != nil {
		sendSkillError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, items)
}

func sendSkillError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, serviceskills.ErrInvalidProvider):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
