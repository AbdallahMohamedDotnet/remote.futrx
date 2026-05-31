package httphandlers

import (
	"errors"
	"net/http"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/codexauth"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type CodexAuthHandler struct {
	login *codexauth.Manager
}

func NewCodexAuthHandler(login *codexauth.Manager) *CodexAuthHandler {
	return &CodexAuthHandler{login: login}
}

func (h *CodexAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/codex/auth-status", h.HandleStatus)
	mux.HandleFunc("/api/codex/login/api-key", h.HandleAPIKey)
}

func (h *CodexAuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	httptransport.SendJSON(w, http.StatusOK, map[string]any{
		"authenticated": h.login.Authenticated(),
	})
}

func (h *CodexAuthHandler) HandleAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		APIKey string `json:"apiKey"`
	}
	if err := readJSONBody(r, &body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.login.LoginWithAPIKey(r.Context(), body.APIKey); err != nil {
		sendCodexLoginError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func sendCodexLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, codexauth.ErrAPIKeyRequired):
		httptransport.SendErr(w, http.StatusBadRequest, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
