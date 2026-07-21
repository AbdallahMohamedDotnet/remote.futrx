package httphandlers

import (
	"errors"
	"net/http"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/codexauth"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type CodexAuthHandler struct {
	login *codexauth.Service
	auth  *serviceauth.Service
}

func NewCodexAuthHandler(login *codexauth.Service, auth *serviceauth.Service) *CodexAuthHandler {
	return &CodexAuthHandler{login: login, auth: auth}
}

func (h *CodexAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/codex/auth-status", h.HandleStatus)
	mux.HandleFunc("/api/codex/login/device", h.HandleDeviceLogin)
}

// HandleStatus stays open to every registered user — the chat composer needs
// to know whether codex is authenticated so it can disable the codex
// provider option if not.
func (h *CodexAuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	httptransport.SendJSON(w, http.StatusOK, h.login.Status())
}

// HandleDeviceLogin is admin-only. Codex auth is host-wide (one credential
// shared across every container), so members shouldn't be able to bump
// each other off the active OAuth session.
func (h *CodexAuthHandler) HandleDeviceLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.auth != nil {
		email, err := callerEmailFromRequest(r, h.auth)
		if err != nil || email == "" {
			httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if ok, _ := h.auth.IsAdmin(r.Context(), email); !ok {
			httptransport.SendErr(w, http.StatusForbidden, "admin only")
			return
		}
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
