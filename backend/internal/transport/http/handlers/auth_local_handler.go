package httphandlers

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	httptransport "github.com/futrx-com/remote.futrx.com/internal/transport/http"
)

type localAuthHandler struct {
	auth   *serviceauth.Service
	logins *localLoginLimiter
}

func (h *localAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/local/claim", h.claim)
	mux.HandleFunc("/auth/local/login", h.login)
}

func (h *localAuthHandler) claim(w http.ResponseWriter, r *http.Request) {
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
	setSessionCookie(w, h.auth, user)
	httptransport.SendJSON(w, http.StatusCreated, h.auth.Status(r.Context(), h.auth.SignSession(user)))
}

func (h *localAuthHandler) login(w http.ResponseWriter, r *http.Request) {
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
	setSessionCookie(w, h.auth, user)
	httptransport.SendJSON(w, http.StatusOK, h.auth.Status(r.Context(), h.auth.SignSession(user)))
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

// localClientIP returns the client address our own reverse proxy observed.
// Caddy fronts the backend on loopback and appends the peer address to any
// X-Forwarded-For the caller supplied, so the rightmost entry is the last hop
// we vouch for; every entry left of it is caller-controlled and must never key
// a rate limiter. Falls back to the peer address for direct (unproxied) access.
func localClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		hops := strings.Split(forwarded, ",")
		if ip := strings.TrimSpace(hops[len(hops)-1]); ip != "" {
			return ip
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
