package httphandlers

import (
	"errors"
	"net/http"
	"strings"

	serviceskills "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/skills"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type SkillHandler struct {
	skills *serviceskills.Service
}

func NewSkillHandler(skills *serviceskills.Service) *SkillHandler {
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

	provider := serviceskills.Provider(strings.TrimSpace(r.URL.Query().Get("provider")))
	if provider == "" {
		provider = serviceskills.ProviderCodex
	}
	items, err := h.skills.List(r.Context(), provider)
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
