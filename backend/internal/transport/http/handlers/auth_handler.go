package httphandlers

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
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

type AuthHandler struct {
	auth   *serviceauth.Service
	access *serviceauth.AccessVerifier
	logins *localLoginLimiter
}

func NewAuthHandler(auth *serviceauth.Service, access *serviceauth.AccessVerifier) *AuthHandler {
	return &AuthHandler{auth: auth, access: access, logins: newLocalLoginLimiter()}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/google/login", h.HandleLogin)
	mux.HandleFunc("/auth/google/callback", h.HandleCallback)
	mux.HandleFunc("/auth/local/claim", h.HandleLocalClaim)
	mux.HandleFunc("/auth/local/login", h.HandleLocalLogin)
	mux.HandleFunc("/auth/logout", h.HandleLogout)
	mux.HandleFunc("/auth/me", h.HandleMe)
	mux.HandleFunc("/auth/verify", h.HandleVerify)
	mux.HandleFunc("/api/admin/auth/google", h.HandleGoogleConfig)
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
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

	authURL, err := h.auth.AuthCodeURL(state)
	if err != nil {
		httptransport.SendErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
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
	user, err := h.auth.LoginGoogle(ctx, code)
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
		if errors.Is(err, serviceauth.ErrLocalAdminPasswordOnly) {
			http.Redirect(w, r, h.auth.BaseURL()+"/?error=admin-password", http.StatusFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.setSessionCookie(w, user)

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

func (h *AuthHandler) HandleLocalClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSONBody(r, &body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	key := localClientIP(r) + "|claim"
	if !h.logins.Allow(key) {
		w.Header().Set("Retry-After", "300")
		httptransport.SendErr(w, http.StatusTooManyRequests, "too many attempts; try again in a few minutes")
		return
	}
	authorizedEmail, _ := callerEmailFromRequest(r, h.auth)
	user, err := h.auth.ClaimLocalAdmin(r.Context(), body.Email, body.Password, authorizedEmail)
	if err != nil {
		h.logins.Failure(key)
		switch {
		case errors.Is(err, serviceauth.ErrLocalAdminAlreadyClaimed):
			httptransport.SendErr(w, http.StatusConflict, err.Error())
		case errors.Is(err, serviceauth.ErrAdminClaimUnauthorized):
			httptransport.SendErr(w, http.StatusForbidden, err.Error())
		case errors.Is(err, serviceauth.ErrPasswordTooShort),
			errors.Is(err, serviceauth.ErrPasswordTooLong):
			httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		default:
			httptransport.SendErr(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	h.logins.Success(key)
	h.setSessionCookie(w, user)
	httptransport.SendJSON(w, http.StatusCreated, h.auth.Status(r.Context(), h.auth.SignSession(user)))
}

func (h *AuthHandler) HandleLocalLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httptransport.SendErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSONBody(r, &body); err != nil {
		httptransport.SendErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	key := localClientIP(r) + "|login"
	if !h.logins.Allow(key) {
		w.Header().Set("Retry-After", "300")
		httptransport.SendErr(w, http.StatusTooManyRequests, "too many attempts; try again in a few minutes")
		return
	}
	user, err := h.auth.LoginLocal(r.Context(), body.Email, body.Password)
	if err != nil {
		h.logins.Failure(key)
		httptransport.SendErr(w, http.StatusUnauthorized, serviceauth.ErrInvalidCredentials.Error())
		return
	}
	h.logins.Success(key)
	h.setSessionCookie(w, user)
	httptransport.SendJSON(w, http.StatusOK, h.auth.Status(r.Context(), h.auth.SignSession(user)))
}

func (h *AuthHandler) HandleGoogleConfig(w http.ResponseWriter, r *http.Request) {
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

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, user serviceauth.User) {
	http.SetCookie(w, &http.Cookie{
		Name: serviceauth.SessionCookieName, Value: h.auth.SignSession(user),
		Path: "/", Domain: h.auth.CookieDomain(),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(serviceauth.SessionDuration().Seconds()),
	})
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

	err := h.access.Verify(r.Context(), sessionCookieValue(r), matchedSlug)
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

func (h *AuthHandler) redirectToLogin(w http.ResponseWriter, r *http.Request) {
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

const localLoginWindow = 5 * time.Minute

type localLoginAttempt struct {
	Failures int
	ResetAt  time.Time
}

type localLoginLimiter struct {
	mu       sync.Mutex
	attempts map[string]localLoginAttempt
}

func newLocalLoginLimiter() *localLoginLimiter {
	return &localLoginLimiter{attempts: make(map[string]localLoginAttempt)}
}

func (l *localLoginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt, ok := l.attempts[key]
	if !ok || time.Now().After(attempt.ResetAt) {
		delete(l.attempts, key)
		return true
	}
	return attempt.Failures < 5
}

func (l *localLoginLimiter) Failure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	attempt := l.attempts[key]
	if attempt.ResetAt.Before(now) {
		attempt = localLoginAttempt{ResetAt: now.Add(localLoginWindow)}
	}
	attempt.Failures++
	l.attempts[key] = attempt
}

func (l *localLoginLimiter) Success(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func localClientIP(r *http.Request) string {
	ip := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return ip
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

func sessionCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(serviceauth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
