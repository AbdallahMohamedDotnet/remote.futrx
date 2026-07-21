package httphandlers

import (
	"errors"
	"net/http"
	"strings"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	serviceskills "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/skills"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type SkillHandler struct {
	skills *serviceskills.Catalog
}

func NewSkillHandler(skills *serviceskills.Catalog) *SkillHandler {
	return &SkillHandler{skills: skills}
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

	items, err := h.skills.List(r.Context(), serviceskills.ListQuery{
		Provider:      serviceskills.Provider(strings.TrimSpace(r.URL.Query().Get("provider"))),
		ProjectID:     serviceproject.ID(strings.TrimSpace(r.URL.Query().Get("projectId"))),
		SessionCookie: sessionCookieValue(r),
	})
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
	case errors.Is(err, serviceskills.ErrProjectLookupUnavailable):
		httptransport.SendErr(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, serviceskills.ErrProjectNotFound):
		httptransport.SendErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, serviceskills.ErrAuthenticationRequired):
		httptransport.SendErr(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, serviceskills.ErrProjectAccessDenied):
		httptransport.SendErr(w, http.StatusForbidden, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
