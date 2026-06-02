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
	// AddBootstrapAdmin adds email as an admin and is invoked only the very
	// first time someone signs in (the box has never been claimed).
	AddBootstrapAdmin(ctx context.Context, email string) error
	Count(ctx context.Context) (int, error)
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

// claimOrAuthorize is the core gate: first-signer claims the box and is
// added to the users table as the bootstrap admin. Every subsequent login
// must be in the users table. The legacy admin.json is also written on
// first-claim for backward compatibility with the old single-admin code
// path (other components still read it via Store.Admin).
func (s *Service) claimOrAuthorize(ctx context.Context, u User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.users != nil {
		count, err := s.users.Count(ctx)
		if err != nil {
			return err
		}
		if count == 0 {
			// First signer: claim the box. Both stores stay consistent.
			if err := s.users.AddBootstrapAdmin(ctx, u.Email); err != nil {
				return err
			}
			return s.store.SaveAdmin(ctx, Admin{
				Email:     u.Email,
				Sub:       u.Sub,
				Name:      u.Name,
				Picture:   u.Picture,
				ClaimedAt: time.Now().UnixMilli(),
			})
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

	// Fallback to the legacy single-admin path when no users directory is
	// wired up. Kept so the package remains usable in isolation (tests).
	admin, err := s.store.Admin(ctx)
	if err != nil {
		return err
	}
	if admin == nil {
		return s.store.SaveAdmin(ctx, Admin{
			Email:     u.Email,
			Sub:       u.Sub,
			Name:      u.Name,
			Picture:   u.Picture,
			ClaimedAt: time.Now().UnixMilli(),
		})
	}
	if !strings.EqualFold(admin.Email, u.Email) {
		return ClaimedError{Email: admin.Email}
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
	if s.users != nil {
		return s.users.IsAdmin(ctx, email)
	}
	admin, err := s.store.Admin(ctx)
	if err != nil {
		return false, err
	}
	return admin != nil && strings.EqualFold(admin.Email, email), nil
}

// IsRegistered returns true if email has a row in the users store. Used by
// the API middleware so members (not just admins) can reach /api/*.
func (s *Service) IsRegistered(ctx context.Context, email string) (bool, error) {
	if s.users != nil {
		return s.users.IsRegistered(ctx, email)
	}
	// No directory: fall back to "is this the legacy admin?" so we don't
	// lock everyone out before users.json exists.
	return s.IsAdmin(ctx, email)
}

func (s *Service) Status(ctx context.Context, cookieValue string) Status {
	admin, _ := s.store.Admin(ctx)
	status := Status{Claimed: admin != nil}
	if admin != nil {
		status.AdminEmail = admin.Email
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
