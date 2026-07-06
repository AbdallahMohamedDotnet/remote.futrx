package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

const sessionDuration = 30 * 24 * time.Hour

// UserDirectory is the interface auth needs into the users store. It's
// satisfied by *user.Service; declared here as a small port so auth doesn't
// import user (avoids an import cycle if user ever needs auth).
type UserDirectory interface {
	IsAdmin(ctx context.Context, email string) (bool, error)
	IsRegistered(ctx context.Context, email string) (bool, error)
	// AddBootstrapAdmin promotes email to the first admin. Called only when
	// users.json is empty (no admins exist yet); subsequent sign-ins go
	// through IsRegistered.
	AddBootstrapAdmin(ctx context.Context, email string) error
	Count(ctx context.Context) (int, error)
	FirstAdmin(ctx context.Context) (*UserDirectoryEntry, error)
}

// UserDirectoryEntry is the minimal projection of a single admin the auth
// service exposes via /auth/me. Status.Claimed is set when one exists,
// Status.AdminEmail is its Email. Currently filled from FirstAdmin (the
// oldest user with role=admin) so the login screen can show "server
// administered by …" without leaking the full directory to anonymous
// callers.
type UserDirectoryEntry struct {
	Email string
}

type Service struct {
	store        Store
	users        UserDirectory
	oauth        OAuthProvider
	baseURL      string
	cookieDomain string
	sessionKey   []byte
	mu           sync.Mutex
}

func NormalizeBaseURL(baseURL string) (string, error) {
	if baseURL == "" {
		return "", errors.New("BASE_URL env var required when auth is enabled (e.g. https://remote.example.com)")
	}
	return strings.TrimRight(baseURL, "/"), nil
}

func New(
	store Store,
	users UserDirectory,
	oauth OAuthProvider,
	baseURL string,
	sessionKey []byte,
) (*Service, error) {
	if store == nil {
		return nil, errors.New("auth store is required")
	}
	if oauth == nil {
		return nil, errors.New("oauth provider is required")
	}
	baseURL, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if len(sessionKey) == 0 {
		return nil, errors.New("session key is required")
	}

	cookieDomain := ""
	if u, err := url.Parse(baseURL); err == nil {
		cookieDomain = u.Hostname()
	}

	return &Service{
		store:        store,
		users:        users,
		oauth:        oauth,
		baseURL:      baseURL,
		cookieDomain: cookieDomain,
		sessionKey:   sessionKey,
	}, nil
}

func (s *Service) BaseURL() string {
	return s.baseURL
}

func (s *Service) CookieDomain() string {
	return s.cookieDomain
}

func (s *Service) AuthCodeURL(state string) string {
	return s.oauth.AuthCodeURL(state)
}

func (s *Service) Login(ctx context.Context, code string) (User, error) {
	user, err := s.oauth.ExchangeUser(ctx, code)
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(user.Email) == "" {
		return User{}, errors.New("OAuth provider returned no email")
	}
	if err := s.claimOrAuthorize(ctx, user); err != nil {
		return User{}, err
	}
	return user, nil
}

// claimOrAuthorize is the post-OAuth membership gate. Three branches:
//  1. users.json empty       — promote the caller to the first admin.
//  2. caller is registered   — allow the sign-in.
//  3. caller not registered  — return NotInvitedError; the handler
//     redirects them to the "ask an admin to
//     add your email" screen.
//
// Mutex-serialized so concurrent first-time sign-ins can't both seed
// themselves as the bootstrap admin.
func (s *Service) claimOrAuthorize(ctx context.Context, u User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.users == nil {
		return errors.New("users directory is not configured")
	}

	count, err := s.users.Count(ctx)
	if err != nil {
		return err
	}
	if count == 0 {
		return s.users.AddBootstrapAdmin(ctx, u.Email)
	}
	ok, err := s.users.IsRegistered(ctx, u.Email)
	if err != nil {
		return err
	}
	if !ok {
		return NotInvitedError{Email: u.Email}
	}
	return nil
}

func (s *Service) SignSession(user User) string {
	now := time.Now()
	return s.sign(Session{
		Email: user.Email,
		Sub:   user.Sub,
		Iat:   now.Unix(),
		Exp:   now.Add(sessionDuration).Unix(),
	})
}

func (s *Service) CurrentSession(cookieValue string) (*Session, error) {
	if cookieValue == "" {
		return nil, errors.New("missing session cookie")
	}
	return s.verify(cookieValue)
}

func (s *Service) IsAdmin(ctx context.Context, email string) (bool, error) {
	if s.users == nil {
		return false, nil
	}
	return s.users.IsAdmin(ctx, email)
}

// IsRegistered returns true if email has a row in the users store. Used by
// the API middleware so members (not just admins) can reach /api/*.
func (s *Service) IsRegistered(ctx context.Context, email string) (bool, error) {
	if s.users == nil {
		return false, nil
	}
	return s.users.IsRegistered(ctx, email)
}

func (s *Service) Status(ctx context.Context, cookieValue string) Status {
	status := Status{}
	if s.users != nil {
		if first, _ := s.users.FirstAdmin(ctx); first != nil {
			status.Claimed = true
			status.AdminEmail = first.Email
		}
	}

	session, err := s.CurrentSession(cookieValue)
	if err != nil {
		return status
	}
	status.Authenticated = true
	status.Email = session.Email
	status.Sub = session.Sub
	status.IsAdmin, _ = s.IsAdmin(ctx, session.Email)
	status.IsRegistered, _ = s.IsRegistered(ctx, session.Email)
	return status
}

func (s *Service) sign(session Session) string {
	body, _ := json.Marshal(session)
	b64 := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, s.sessionKey)
	mac.Write([]byte(b64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig
}

func (s *Service) verify(value string) (*Session, error) {
	parts := strings.SplitN(value, ".", 2)
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
	var session Session
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, err
	}
	if time.Now().Unix() > session.Exp {
		return nil, errors.New("expired")
	}
	return &session, nil
}

func SessionDuration() time.Duration {
	return sessionDuration
}
