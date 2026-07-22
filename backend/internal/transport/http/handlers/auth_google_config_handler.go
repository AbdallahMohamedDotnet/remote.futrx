package httphandlers

import (
	"net/http"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
)

type googleConfigHandler struct {
	auth *serviceauth.Service
}

func (h *googleConfigHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/auth/google", h.serve)
}

func (h *googleConfigHandler) serve(w http.ResponseWriter, r *http.Request) {
	email, err := callerEmailFromRequest(r, h.auth)
	if err != nil || email == "" {
		httptransport.SendErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if admin, _ := h.auth.IsAdmin(r.Context(), email); !admin {
		httptransport.SendErr(w, http.StatusForbidden, "admin only")
		return
	}

	response := func() map[string]any {
		return map[string]any{
			"configured":  h.auth.GoogleOAuthEnabled(),
			"clientId":    h.auth.GoogleClientID(),
			"redirectUrl": h.auth.BaseURL() + "/auth/google/callback",
		}
	}
	switch r.Method {
	case http.MethodGet:
		httptransport.SendJSON(w, http.StatusOK, response())
	case http.MethodPut:
		var body struct {
			ClientID     string `json:"clientId"`
			ClientSecret string `json:"clientSecret"`
		}
		if err := readJSONBody(r, &body); err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, "invalid request")
			return
		}
		err := h.auth.ConfigureGoogleOAuth(r.Context(), serviceauth.OAuthConfig{
			GoogleClientID: body.ClientID, GoogleClientSecret: body.ClientSecret,
		})
		if err != nil {
			httptransport.SendErr(w, http.StatusBadRequest, err.Error())
			return
		}
		httptransport.SendJSON(w, http.StatusOK, response())
	default:
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
