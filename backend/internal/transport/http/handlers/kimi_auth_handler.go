package httphandlers

import (
	"net/http"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/kimiauth"
	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type KimiAuthHandler struct {
	login *kimiauth.Manager
	auth  *serviceauth.Service
}

func NewKimiAuthHandler(login *kimiauth.Manager, auth *serviceauth.Service) *KimiAuthHandler {
	return &KimiAuthHandler{login: login, auth: auth}
}

func (h *KimiAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/kimi/auth-status", h.HandleStatus)
	mux.HandleFunc("/api/kimi/login/device", h.HandleDeviceLogin)
}

// HandleStatus stays open to every registered user — the settings/composer
// surfaces need to know whether Kimi is authenticated.
func (h *KimiAuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	httptransport.SendJSON(w, http.StatusOK, h.login.Status())
}

// HandleDeviceLogin is admin-only. Kimi auth is host-wide (one subscription
// credential shared across every container), so members shouldn't be able to
// bump each other off the active OAuth session.
func (h *KimiAuthHandler) HandleDeviceLogin(w http.ResponseWriter, r *http.Request) {
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
		httptransport.SendErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	httptransport.SendJSON(w, http.StatusOK, state)
}
