package auth

import (
	"context"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/httpserver"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// oauthConfig is persisted at data/oauth.json. If the file is missing, the
// service runs with no auth — useful for local dev and existing deployments
// that haven't enabled auth yet.
type oauthConfig struct {
	GoogleClientID     string `json:"googleClientId"`
	GoogleClientSecret string `json:"googleClientSecret"`
}

// adminInfo is persisted at data/admin.json. Existence == server is claimed.
type adminInfo struct {
	Email     string `json:"email"`
	Sub       string `json:"sub"`
	Name      string `json:"name,omitempty"`
	Picture   string `json:"picture,omitempty"`
	ClaimedAt int64  `json:"claimedAt"`
}

// AuthService — Google OAuth + first-login-wins admin + HMAC-signed session
// cookies. Stateless: no server-side session table; cookie carries everything.
//
// admin.json is the source of truth for who the admin is; we DO NOT cache it
// in memory. That way `rm data/admin.json` (followed by another Google login)
// is enough to reassign admin — no restart needed. The file is small (<1 KB)
// and the OS page cache means each read is effectively in-memory.
type AuthService struct {
	dataDir      string
	baseURL      string
	cookieDomain string // hostname extracted from baseURL; cookies set with
	//                    Domain=this so they're valid on subdomains too
	//                    (e.g. code.remote.futrx.dev when baseURL is
	//                    https://remote.futrx.dev).
	oauth      *oauth2.Config
	sessionKey []byte
	mu         sync.Mutex // serializes read-modify-write of admin.json
}

// readAdmin returns the admin from disk or nil if unclaimed.
func (s *AuthService) readAdmin() *adminInfo {
	data, err := os.ReadFile(filepath.Join(s.dataDir, "admin.json"))
	if err != nil {
		return nil
	}
	var a adminInfo
	if err := json.Unmarshal(data, &a); err != nil || a.Email == "" {
		return nil
	}
	return &a
}

// writeAdmin persists the admin record to disk with tight permissions.
func (s *AuthService) writeAdmin(a *adminInfo) error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dataDir, "admin.json"), data, 0o600)
}

// LoadAuthService returns nil, nil if no oauth.json exists (auth disabled).
func LoadAuthService(dataDir, baseURL string) (*AuthService, error) {
	cfgBytes, err := os.ReadFile(filepath.Join(dataDir, "oauth.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // auth disabled
		}
		return nil, fmt.Errorf("read oauth.json: %w", err)
	}
	var cfg oauthConfig
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		return nil, fmt.Errorf("parse oauth.json: %w", err)
	}
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		return nil, errors.New("oauth.json present but missing googleClientId or googleClientSecret")
	}
	if baseURL == "" {
		return nil, errors.New("BASE_URL env var required when auth is enabled (e.g. https://remote.example.com)")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Parse the cookie domain (hostname) from BASE_URL. Setting Domain on the
	// session cookie makes it valid for the host AND all subdomains, which
	// lets a sibling service like code.remote.futrx.dev verify auth via the
	// same cookie behind Caddy forward_auth.
	cookieDomain := ""
	if u, err := url.Parse(baseURL); err == nil {
		cookieDomain = u.Hostname()
	}

	// Load or generate the HMAC session-signing key.
	keyPath := filepath.Join(dataDir, "session.key")
	sessionKey, err := os.ReadFile(keyPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read session.key: %w", err)
		}
		sessionKey = make([]byte, 32)
		if _, err := crand.Read(sessionKey); err != nil {
			return nil, fmt.Errorf("gen session key: %w", err)
		}
		if err := os.WriteFile(keyPath, sessionKey, 0o600); err != nil {
			return nil, fmt.Errorf("write session.key: %w", err)
		}
	}

	// admin.json is read on every check, not cached, so rm data/admin.json
	// instantly unclaims the server (next Google login re-claims).

	return &AuthService{
		dataDir:      dataDir,
		baseURL:      baseURL,
		cookieDomain: cookieDomain,
		oauth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  baseURL + "/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		sessionKey: sessionKey,
	}, nil
}

func (s *AuthService) Routes() httpserver.AuthRoutes {
	return httpserver.AuthRoutes{
		Login:      s.handleLogin,
		Callback:   s.handleCallback,
		Logout:     s.handleLogout,
		Me:         s.handleMe,
		Verify:     s.handleVerify,
		Middleware: s.Middleware,
	}
}

// handleVerify is called by an edge reverse-proxy (Caddy forward_auth) on
// sibling subdomains to gate them on this server's admin session.
//
//   - 200 OK with an empty body  → the request has a valid admin session;
//                                  proxy should serve the upstream.
//   - 302 to baseURL/            → no/invalid session; bounce the user to
//                                  the main login. Once they sign in there,
//                                  the cookie is scoped to .baseDomain so a
//                                  subsequent request to the sibling
//                                  subdomain carries it.
//
// We deliberately don't propagate a return_to URL through OAuth state today
// — keeps the wire surface tiny. The user manually returns to the sibling
// after login. If that ever rankles, add return_to here + plumb through state.
func (s *AuthService) handleVerify(w http.ResponseWriter, r *http.Request) {
	p := s.currentUser(r)
	if p != nil && s.isAdmin(p.Email) {
		w.WriteHeader(http.StatusOK)
		return
	}
	target := s.baseURL + "/"
	if target == "/" {
		// No BASE_URL configured — fall back to a plain 401.
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// --- session cookie ---------------------------------------------------------

const (
	sessionCookieName = "remote_session"
	stateCookieName   = "remote_oauth_state"
	sessionDuration   = 30 * 24 * time.Hour
)

type sessionPayload struct {
	Email string `json:"email"`
	Sub   string `json:"sub"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

func (s *AuthService) signCookie(p sessionPayload) string {
	body, _ := json.Marshal(p)
	b64 := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte(b64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig
}

func (s *AuthService) verifyCookie(v string) (*sessionPayload, error) {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed")
	}
	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(want)) {
		return nil, errors.New("bad signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var p sessionPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	if time.Now().Unix() > p.Exp {
		return nil, errors.New("expired")
	}
	return &p, nil
}

func (s *AuthService) currentUser(r *http.Request) *sessionPayload {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	p, err := s.verifyCookie(c.Value)
	if err != nil {
		return nil
	}
	return p
}

func (s *AuthService) isAdmin(email string) bool {
	a := s.readAdmin()
	return a != nil && strings.EqualFold(a.Email, email)
}

// --- HTTP handlers ----------------------------------------------------------

func (s *AuthService) handleLogin(w http.ResponseWriter, r *http.Request) {
	stateBytes := make([]byte, 16)
	if _, err := crand.Read(stateBytes); err != nil {
		http.Error(w, "rand", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	http.SetCookie(w, &http.Cookie{
		Name: stateCookieName, Value: state,
		Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: 600,
	})
	url := s.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *AuthService) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify OAuth state.
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "bad oauth state — try logging in again", http.StatusBadRequest)
		return
	}
	// Best-effort clear of state cookie.
	http.SetCookie(w, &http.Cookie{
		Name: stateCookieName, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tok, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	client := s.oauth.Client(ctx, tok)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "userinfo request failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		http.Error(w, "userinfo non-200: "+resp.Status, http.StatusInternalServerError)
		return
	}

	var userInfo struct {
		Sub     string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "userinfo parse: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if userInfo.Email == "" {
		http.Error(w, "Google returned no email", http.StatusInternalServerError)
		return
	}

	// First-login-wins claim, or admin match. Read admin.json fresh under the
	// lock so two simultaneous "claim" attempts can't both succeed.
	s.mu.Lock()
	admin := s.readAdmin()
	if admin == nil {
		newAdmin := &adminInfo{
			Email: userInfo.Email, Sub: userInfo.Sub,
			Name: userInfo.Name, Picture: userInfo.Picture,
			ClaimedAt: time.Now().UnixMilli(),
		}
		if err := s.writeAdmin(newAdmin); err != nil {
			s.mu.Unlock()
			http.Error(w, "write admin.json: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("auth: claimed by %s (sub=%s)", userInfo.Email, userInfo.Sub)
	} else if !strings.EqualFold(admin.Email, userInfo.Email) {
		claimedBy := admin.Email
		s.mu.Unlock()
		http.Error(w, fmt.Sprintf(
			"This server is claimed by %s. If you should have access, ask them to add you — or SSH in and remove data/admin.json to reset.",
			claimedBy,
		), http.StatusForbidden)
		return
	}
	s.mu.Unlock()

	// Issue session cookie.
	now := time.Now()
	cookie := s.signCookie(sessionPayload{
		Email: userInfo.Email, Sub: userInfo.Sub,
		Iat: now.Unix(), Exp: now.Add(sessionDuration).Unix(),
	})
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: cookie,
		Path: "/", Domain: s.cookieDomain,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionDuration.Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *AuthService) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear both the host-only and the .domain-scoped cookie so legacy sessions
	// (pre-cross-subdomain) also expire cleanly.
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Path: "/", Domain: s.cookieDomain, MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *AuthService) handleMe(w http.ResponseWriter, r *http.Request) {
	p := s.currentUser(r)
	admin := s.readAdmin()
	claimed := admin != nil
	var adminEmail string
	if claimed {
		adminEmail = admin.Email
	}

	if p == nil {
		httpserver.SendJSON(w, 200, map[string]any{
			"authenticated": false,
			"claimed":       claimed,
			"adminEmail":    adminEmail, // shown on lockout to know who claimed it
		})
		return
	}
	httpserver.SendJSON(w, 200, map[string]any{
		"authenticated": true,
		"claimed":       claimed,
		"adminEmail":    adminEmail,
		"email":         p.Email,
		"sub":           p.Sub,
		"isAdmin":       s.isAdmin(p.Email),
	})
}

// Middleware gates /api/* and /ws/* on a valid session + admin match.
// Static assets and the auth flow itself are always public — the SPA
// handles displaying the login screen client-side when /auth/me returns
// authenticated:false.
func (s *AuthService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Public paths: everything except API / WS calls.
		if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws") {
			next.ServeHTTP(w, r)
			return
		}
		// /auth/me is the only /api endpoint that's public (so the SPA can
		// detect logged-out state). Everything else under /api/ is gated.
		if path == "/auth/me" || strings.HasPrefix(path, "/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		p := s.currentUser(r)
		if p == nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if !s.isAdmin(p.Email) {
			http.Error(w, "forbidden — not the admin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
