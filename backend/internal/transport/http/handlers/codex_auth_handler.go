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
	mux.HandleFunc("/api/codex/login/device", h.HandleDeviceLogin)
}

func (h *CodexAuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	httptransport.SendJSON(w, http.StatusOK, h.login.Status())
}

func (h *CodexAuthHandler) HandleDeviceLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	state, err := h.login.StartDeviceLogin(r.Context())
	if err != nil {
		sendCodexLoginError(w, err)
		return
	}
	httptransport.SendJSON(w, http.StatusOK, state)
}

func sendCodexLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, codexauth.ErrCodexNotFound):
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	default:
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
	}
}
