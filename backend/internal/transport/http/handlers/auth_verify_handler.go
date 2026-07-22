package httphandlers

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
)

var projectVerifyHostPattern = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)--(\d{4,5})\.dev\.(.+)$`)

type authVerifyHandler struct {
	auth   *serviceauth.Service
	access *serviceauth.AccessVerifier
}

func (h *authVerifyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/verify", h.verify)
}

func (h *authVerifyHandler) verify(w http.ResponseWriter, r *http.Request) {
	host := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Host")))
	matchedSlug := ""
	if match := projectVerifyHostPattern.FindStringSubmatch(host); match != nil {
		base := strings.ToLower(strings.TrimSpace(baseHost(h.auth.BaseURL())))
		if base != "" && match[3] == base {
			matchedSlug = match[1]
		}
	}

	err := h.access.Verify(r.Context(), httptransport.SessionCookieValue(r), matchedSlug)
	switch {
	case errors.Is(err, serviceauth.ErrAuthenticationRequired):
		h.redirectToLogin(w, r)
	case errors.Is(err, serviceauth.ErrProjectNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, serviceauth.ErrProjectAccessDenied),
		errors.Is(err, serviceauth.ErrAccountNotAuthorized):
		http.Error(w, err.Error(), http.StatusForbidden)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (h *authVerifyHandler) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	base := h.auth.BaseURL()
	if base == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	loginURL := base + "/"
	if returnTo := reconstructOriginalURL(r); returnTo != "" && isSafeReturnTo(returnTo, base) {
		loginURL += "?return_to=" + url.QueryEscape(returnTo)
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}
