package httphandlers

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

// returnToCookieName is the short-lived cookie carrying the post-login redirect
// target through the OAuth round-trip. Set in HandleLogin, read+cleared in
// HandleCallback. 10-minute lifetime: enough time to finish a Google sign-in
// but not so long that a stale value from a previous flow leaks.
const returnToCookieName = "return_to"

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

	// If a return_to was passed in (typically by HandleVerify bouncing an
	// unauth request from a *.dev subdomain), validate it and stash it in a
	// short-lived cookie. The OAuth callback reads it and redirects there
	// instead of dropping the user on the main site.
	if rt := r.URL.Query().Get("return_to"); rt != "" && isSafeReturnTo(rt, h.auth.BaseURL()) {
		http.SetCookie(w, &http.Cookie{
			Name: returnToCookieName, Value: rt,
			Path: "/", Domain: h.auth.CookieDomain(),
			HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
			MaxAge: 600,
		})
	}

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

	// Default landing is the main site root. If a validated return_to cookie
	// is present (set in HandleLogin), redirect there instead so the user
	// lands on whichever subdomain they originally tried to reach.
	target := "/"
	if c, err := r.Cookie(returnToCookieName); err == nil && isSafeReturnTo(c.Value, h.auth.BaseURL()) {
		target = c.Value
	}
	// Clear the return_to cookie in either case so a stale value never wins.
	http.SetCookie(w, &http.Cookie{
		Name: returnToCookieName, Path: "/", Domain: h.auth.CookieDomain(), MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, target, http.StatusFound)
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
	base := h.auth.BaseURL()
	if base == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	// Reconstruct the URL the user originally tried to reach so the OAuth
	// callback can bounce them back there. Caddy's forward_auth sets these
	// X-Forwarded-* headers on its subrequest to /auth/verify; reverse_proxy
	// fills in X-Forwarded-Host from the original Host.
	loginURL := base + "/auth/google/login"
	if returnTo := reconstructOriginalURL(r); returnTo != "" && isSafeReturnTo(returnTo, base) {
		loginURL += "?return_to=" + url.QueryEscape(returnTo)
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

// reconstructOriginalURL builds the full URL the client was hitting before
// Caddy proxied to /auth/verify. Returns "" if the required X-Forwarded-*
// headers aren't present (i.e. /auth/verify was hit directly, not via
// forward_auth).
func reconstructOriginalURL(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	uri := r.Header.Get("X-Forwarded-Uri")
	if host == "" || uri == "" {
		return ""
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "https"
	}
	return proto + "://" + host + uri
}

// isSafeReturnTo prevents open-redirect: only allow URLs whose host equals
// the configured base host OR ends with "." + base host (i.e. any subdomain
// we ourselves serve). https-only.
func isSafeReturnTo(rawURL, base string) bool {
	if rawURL == "" || len(rawURL) > 2048 {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return false
	}
	bu, err := url.Parse(base)
	if err != nil || bu.Host == "" {
		return false
	}
	return u.Host == bu.Host || strings.HasSuffix(u.Host, "."+bu.Host)
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
