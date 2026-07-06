package httphandlers

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"

	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	httptransport "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/transport/http"
)

// returnToCookieName is the short-lived cookie carrying the post-login redirect
// target through the OAuth round-trip. Set in HandleLogin, read+cleared in
// HandleCallback. 10-minute lifetime: enough time to finish a Google sign-in
// but not so long that a stale value from a previous flow leaks.
const returnToCookieName = "return_to"

// projectVerifyHostPattern matches the per-project preview hostnames Caddy
// proxies (`<slug>--<port>.dev.<base-host>`). HandleVerify consults this to
// gate project subdomains on membership instead of just authentication.
var projectVerifyHostPattern = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)--(\d{4,5})\.dev\.(.+)$`)

// ProjectAccessGate is what AuthHandler needs from the project service to
// gate per-project subdomains. Declared as a small interface so the handler
// can be used in tests without standing up a full *project.Service.
type ProjectAccessGate interface {
	GetBySlug(ctx context.Context, slug string) (serviceproject.Meta, error)
	HasAccess(ctx context.Context, id serviceproject.ID, email string) (bool, error)
}

type AuthHandler struct {
	auth     *serviceauth.Service
	projects ProjectAccessGate
}

func NewAuthHandler(auth *serviceauth.Service, projects ProjectAccessGate) *AuthHandler {
	return &AuthHandler{auth: auth, projects: projects}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/google/login", h.HandleLogin)
	mux.HandleFunc("/auth/google/callback", h.HandleCallback)
	mux.HandleFunc("/auth/logout", h.HandleLogout)
	mux.HandleFunc("/auth/me", h.HandleMe)
	mux.HandleFunc("/auth/verify", h.HandleVerify)
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
		// Bounce uninvited users back to the login screen with a query
		// param the LoginScreen can render verbatim. This keeps the OAuth
		// flow from looking like a hard 403 in the browser.
		var notInvited serviceauth.NotInvitedError
		if errors.As(err, &notInvited) {
			base := h.auth.BaseURL()
			loginURL := base + "/?error=not-invited&email=" + url.QueryEscape(notInvited.Email)
			http.Redirect(w, r, loginURL, http.StatusFound)
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
	session, sessionErr := h.auth.CurrentSession(sessionCookieValue(r))
	authenticated := sessionErr == nil && session != nil

	// Determine whether the request is for a per-project preview subdomain
	// (`<slug>--<port>.dev.<base-host>`) so we can gate on project
	// membership instead of plain authentication.
	host := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Host")))
	matchedSlug := ""
	if m := projectVerifyHostPattern.FindStringSubmatch(host); m != nil {
		// Compare the trailing base host against our configured baseURL host.
		base := strings.ToLower(strings.TrimSpace(baseHost(h.auth.BaseURL())))
		if base != "" && m[3] == base {
			matchedSlug = m[1]
		}
	}

	if matchedSlug != "" && h.projects != nil {
		if !authenticated {
			h.redirectToLogin(w, r)
			return
		}
		proj, err := h.projects.GetBySlug(r.Context(), matchedSlug)
		if err != nil {
			// Unknown slug or DB error: treat as not-found to be safe.
			http.Error(w, "no such project", http.StatusNotFound)
			return
		}
		// Admins always pass; otherwise check membership.
		isAdmin, _ := h.auth.IsAdmin(r.Context(), session.Email)
		if !isAdmin {
			ok, _ := h.projects.HasAccess(r.Context(), proj.ID, session.Email)
			if !ok {
				http.Error(w, "forbidden - not a member of this project", http.StatusForbidden)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Non-project request (main UI, code-server, etc.): any registered user
	// passes. Unauthenticated → bounce to login.
	if !authenticated {
		h.redirectToLogin(w, r)
		return
	}
	if ok, _ := h.auth.IsRegistered(r.Context(), session.Email); ok {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Error(w, "account not authorized", http.StatusForbidden)
}

func (h *AuthHandler) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	base := h.auth.BaseURL()
	if base == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	loginURL := base + "/auth/google/login"
	if returnTo := reconstructOriginalURL(r); returnTo != "" && isSafeReturnTo(returnTo, base) {
		loginURL += "?return_to=" + url.QueryEscape(returnTo)
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

// baseHost returns the host portion of a base URL (without scheme, without
// trailing port). Returns "" on parse failure.
func baseHost(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Hostname()
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
		ok, _ := h.auth.IsRegistered(r.Context(), session.Email)
		if !ok {
			http.Error(w, "account not authorized", http.StatusUnauthorized)
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
