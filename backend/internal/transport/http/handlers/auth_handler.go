package httphandlers

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

type AuthHandler struct {
	auth *serviceauth.Service
}

func NewAuthHandler(auth *serviceauth.Service) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Routes() httptransport.AuthRoutes {
	return httptransport.AuthRoutes{
		Login:      h.HandleLogin,
		Callback:   h.HandleCallback,
		Logout:     h.HandleLogout,
		Me:         h.HandleMe,
		Verify:     h.HandleVerify,
		Middleware: h.Middleware,
	}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	stateBytes := make([]byte, 16)
	if _, err := crand.Read(stateBytes); err != nil {
		http.Error(w, "rand", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.StateCookieName, Value: state,
		Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: 600,
	})
	http.Redirect(w, r, h.auth.AuthCodeURL(state), http.StatusFound)
}

func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(serviceauth.StateCookieName)
	if err != nil || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "bad oauth state - try logging in again", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.StateCookieName, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user, err := h.auth.Login(ctx, code)
	if err != nil {
		var claimed serviceauth.ClaimedError
		if errors.As(err, &claimed) {
			http.Error(w, fmt.Sprintf(
				"This server is claimed by %s. If you should have access, ask them to add you - or SSH in and remove data/admin.json to reset.",
				claimed.Email,
			), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.SessionCookieName, Value: h.auth.SignSession(user),
		Path: "/", Domain: h.auth.CookieDomain(),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(serviceauth.SessionDuration().Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.SessionCookieName, Path: "/", Domain: h.auth.CookieDomain(), MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.SessionCookieName, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	httptransport.SendJSON(w, http.StatusOK, h.auth.Status(r.Context(), sessionCookieValue(r)))
}

func (h *AuthHandler) HandleVerify(w http.ResponseWriter, r *http.Request) {
	session, err := h.auth.CurrentSession(sessionCookieValue(r))
	if err == nil {
		if ok, _ := h.auth.IsAdmin(r.Context(), session.Email); ok {
			w.WriteHeader(http.StatusOK)
			return
		}
	}
	target := h.auth.BaseURL() + "/"
	if target == "/" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws") {
			next.ServeHTTP(w, r)
			return
		}
		if path == "/auth/me" || strings.HasPrefix(path, "/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		session, err := h.auth.CurrentSession(sessionCookieValue(r))
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if ok, _ := h.auth.IsAdmin(r.Context(), session.Email); !ok {
			http.Error(w, "forbidden - not the admin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sessionCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(serviceauth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
