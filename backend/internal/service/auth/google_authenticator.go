package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// GoogleAuthenticator owns Google OAuth provider configuration and the invited
// user login policy. Service delegates to it to preserve the public facade.
type GoogleAuthenticator struct {
	store        OAuthConfigStore
	users        UserDirectory
	factory      OAuthProviderFactory
	baseURL      string
	isLocalAdmin func(string) bool

	mu       sync.RWMutex
	provider OAuthProvider
	config   OAuthConfig
}

func newGoogleAuthenticator(
	ctx context.Context,
	store OAuthConfigStore,
	users UserDirectory,
	factory OAuthProviderFactory,
	baseURL string,
	isLocalAdmin func(string) bool,
) (*GoogleAuthenticator, error) {
	config, err := store.OAuthConfig(ctx)
	if err != nil && !errors.Is(err, ErrOAuthConfigNotFound) {
		return nil, err
	}
	authenticator := &GoogleAuthenticator{
		store:        store,
		users:        users,
		factory:      factory,
		baseURL:      baseURL,
		isLocalAdmin: isLocalAdmin,
	}
	if err == nil && config.GoogleClientID != "" && config.GoogleClientSecret != "" {
		authenticator.config = config
		authenticator.provider = factory(
			config.GoogleClientID,
			config.GoogleClientSecret,
			baseURL+"/auth/google/callback",
		)
	}
	return authenticator, nil
}

func (a *GoogleAuthenticator) authCodeURL(state string) (string, error) {
	a.mu.RLock()
	provider := a.provider
	a.mu.RUnlock()
	if provider == nil {
		return "", ErrGoogleOAuthDisabled
	}
	return provider.AuthCodeURL(state), nil
}

func (a *GoogleAuthenticator) login(ctx context.Context, code string) (User, error) {
	a.mu.RLock()
	provider := a.provider
	a.mu.RUnlock()
	if provider == nil {
		return User{}, ErrGoogleOAuthDisabled
	}
	user, err := provider.ExchangeUser(ctx, code)
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(user.Email) == "" {
		return User{}, errors.New("OAuth provider returned no email")
	}
	if err := a.authorize(ctx, user); err != nil {
		return User{}, err
	}
	return user, nil
}

// Google is only an invited-user login. It can never claim the server or
// silently promote the first Google account to administrator.
func (a *GoogleAuthenticator) authorize(ctx context.Context, user User) error {
	if a.users == nil {
		return errors.New("users directory is not configured")
	}
	if a.isLocalAdmin(user.Email) {
		return ErrLocalAdminPasswordOnly
	}
	registered, err := a.users.IsRegistered(ctx, user.Email)
	if err != nil {
		return err
	}
	if !registered {
		return NotInvitedError{Email: user.Email}
	}
	return nil
}

func (a *GoogleAuthenticator) configure(ctx context.Context, config OAuthConfig) error {
	config.GoogleClientID = strings.TrimSpace(config.GoogleClientID)
	config.GoogleClientSecret = strings.TrimSpace(config.GoogleClientSecret)
	if config.GoogleClientID == "" || config.GoogleClientSecret == "" ||
		len(config.GoogleClientID) > 1024 || len(config.GoogleClientSecret) > 4096 {
		return ErrInvalidOAuthConfig
	}
	if err := a.store.SaveOAuthConfig(ctx, config); err != nil {
		return err
	}
	provider := a.factory(config.GoogleClientID, config.GoogleClientSecret, a.baseURL+"/auth/google/callback")
	a.mu.Lock()
	a.config = config
	a.provider = provider
	a.mu.Unlock()
	return nil
}

func (a *GoogleAuthenticator) enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.provider != nil
}

func (a *GoogleAuthenticator) clientID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.GoogleClientID
}
