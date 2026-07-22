package auth

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var localAdminEmailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

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
	google       *GoogleAuthenticator
	baseURL      string
	cookieDomain string
	sessions     *SessionCodec

	localMu           sync.RWMutex
	localAdmin        *LocalAdminCredential
	dummyPasswordHash string
	claimMu           sync.Mutex
}

func NormalizeBaseURL(baseURL string) (string, error) {
	if baseURL == "" {
		return "", errors.New("BASE_URL env var required when auth is enabled (e.g. https://remote.example.com)")
	}
	return strings.TrimRight(baseURL, "/"), nil
}

func New(
	ctx context.Context,
	store Store,
	users UserDirectory,
	oauthFactory OAuthProviderFactory,
	baseURL string,
	sessionKey []byte,
) (*Service, error) {
	if store == nil {
		return nil, errors.New("auth store is required")
	}
	if oauthFactory == nil {
		return nil, errors.New("OAuth provider factory is required")
	}
	baseURL, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if len(sessionKey) == 0 {
		return nil, errors.New("session key is required")
	}
	localAdmin, err := store.LocalAdmin(ctx)
	if err != nil {
		return nil, err
	}
	var service *Service
	google, err := newGoogleAuthenticator(ctx, store, users, oauthFactory, baseURL, func(email string) bool {
		return service != nil && service.IsLocalAdmin(email)
	})
	if err != nil {
		return nil, err
	}
	dummyHash, err := HashPassword("invalid-password-placeholder")
	if err != nil {
		return nil, err
	}

	cookieDomain := ""
	if u, err := url.Parse(baseURL); err == nil {
		cookieDomain = u.Hostname()
	}

	service = &Service{
		store:             store,
		users:             users,
		google:            google,
		baseURL:           baseURL,
		cookieDomain:      cookieDomain,
		sessions:          newSessionCodec(sessionKey),
		localAdmin:        localAdmin,
		dummyPasswordHash: dummyHash,
	}
	return service, nil
}

func (s *Service) BaseURL() string {
	return s.baseURL
}

func (s *Service) CookieDomain() string {
	return s.cookieDomain
}

func (s *Service) AuthCodeURL(state string) (string, error) {
	return s.google.authCodeURL(state)
}

func (s *Service) LoginGoogle(ctx context.Context, code string) (User, error) {
	return s.google.login(ctx, code)
}

func (s *Service) ClaimLocalAdmin(ctx context.Context, email, password, authorizedEmail string) (User, error) {
	email = normalizeEmail(email)
	if !localAdminEmailPattern.MatchString(email) {
		return User{}, errors.New("valid admin email is required")
	}

	s.claimMu.Lock()
	defer s.claimMu.Unlock()
	s.localMu.RLock()
	alreadyClaimed := s.localAdmin != nil
	s.localMu.RUnlock()
	if alreadyClaimed {
		return User{}, ErrLocalAdminAlreadyClaimed
	}

	if s.users == nil {
		return User{}, errors.New("users directory is not configured")
	}
	if first, err := s.users.FirstAdmin(ctx); err != nil {
		return User{}, err
	} else if first != nil {
		authorizedEmail = normalizeEmail(authorizedEmail)
		isAdmin, authErr := s.users.IsAdmin(ctx, authorizedEmail)
		if authErr != nil {
			return User{}, authErr
		}
		if !isAdmin || authorizedEmail != email || email != normalizeEmail(first.Email) {
			return User{}, ErrAdminClaimUnauthorized
		}
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	credential := LocalAdminCredential{Email: email, PasswordHash: passwordHash}
	if err := s.store.CreateLocalAdmin(ctx, credential); err != nil {
		return User{}, err
	}
	s.localMu.Lock()
	s.localAdmin = &credential
	s.localMu.Unlock()

	registered, err := s.users.IsRegistered(ctx, email)
	if err != nil {
		return User{}, err
	}
	if !registered {
		if err := s.users.AddBootstrapAdmin(ctx, email); err != nil {
			return User{}, err
		}
	}
	return localAdminUser(email), nil
}

func (s *Service) LoginLocal(_ context.Context, email, password string) (User, error) {
	s.localMu.RLock()
	credential := s.localAdmin
	hash := s.dummyPasswordHash
	if credential != nil {
		copy := *credential
		credential = &copy
		hash = copy.PasswordHash
	}
	s.localMu.RUnlock()

	passwordOK, err := VerifyPassword(hash, password)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}
	emailOK := credential != nil && normalizeEmail(email) == credential.Email
	if !emailOK || !passwordOK {
		return User{}, ErrInvalidCredentials
	}
	return localAdminUser(credential.Email), nil
}

func (s *Service) ConfigureGoogleOAuth(ctx context.Context, cfg OAuthConfig) error {
	return s.google.configure(ctx, cfg)
}

func (s *Service) GoogleOAuthEnabled() bool {
	return s.google.enabled()
}

func (s *Service) GoogleClientID() string {
	return s.google.clientID()
}

func (s *Service) LocalAdminConfigured() bool {
	s.localMu.RLock()
	defer s.localMu.RUnlock()
	return s.localAdmin != nil
}

func (s *Service) IsLocalAdmin(email string) bool {
	s.localMu.RLock()
	defer s.localMu.RUnlock()
	return s.localAdmin != nil && s.localAdmin.Email == normalizeEmail(email)
}

func (s *Service) SignSession(user User) string {
	return s.sessions.sign(user)
}

func (s *Service) CurrentSession(cookieValue string) (*Session, error) {
	if cookieValue == "" {
		return nil, errors.New("missing session cookie")
	}
	session, err := s.sessions.verify(cookieValue)
	if err != nil {
		return nil, err
	}
	// Once the local administrator exists, invalidate any older Google-backed
	// sessions for that email. The owner account must remain password-only;
	// invited users may continue using Google.
	if s.IsLocalAdmin(session.Email) && session.Sub != "local-admin" {
		return nil, ErrLocalAdminPasswordOnly
	}
	return session, nil
}

func (s *Service) IsAdmin(ctx context.Context, email string) (bool, error) {
	if s.IsLocalAdmin(email) {
		return true, nil
	}
	if s.users == nil {
		return false, nil
	}
	return s.users.IsAdmin(ctx, email)
}

// IsRegistered returns true if email has a row in the users store. Used by
// the API middleware so members (not just admins) can reach /api/*.
func (s *Service) IsRegistered(ctx context.Context, email string) (bool, error) {
	if s.IsLocalAdmin(email) {
		return true, nil
	}
	if s.users == nil {
		return false, nil
	}
	return s.users.IsRegistered(ctx, email)
}

func (s *Service) Status(ctx context.Context, cookieValue string) Status {
	status := Status{
		LocalAdminConfigured: s.LocalAdminConfigured(),
		GoogleOAuthEnabled:   s.GoogleOAuthEnabled(),
		GoogleClientID:       s.GoogleClientID(),
	}
	s.localMu.RLock()
	if s.localAdmin != nil {
		status.Claimed = true
		status.AdminEmail = s.localAdmin.Email
	}
	s.localMu.RUnlock()
	if !status.Claimed && s.users != nil {
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

func localAdminUser(email string) User {
	return User{Email: normalizeEmail(email), Sub: "local-admin"}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func SessionDuration() time.Duration {
	return sessionDuration
}
