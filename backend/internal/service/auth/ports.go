package auth

import "context"

type Store interface {
	OAuthConfig(context.Context) (OAuthConfig, error)
	SaveOAuthConfig(context.Context, OAuthConfig) error
	LocalAdmin(context.Context) (*LocalAdminCredential, error)
	CreateLocalAdmin(context.Context, LocalAdminCredential) error
	SessionKey(context.Context) ([]byte, error)
}

type OAuthProvider interface {
	AuthCodeURL(state string) string
	ExchangeUser(ctx context.Context, code string) (User, error)
}

type OAuthProviderFactory func(clientID, clientSecret, redirectURL string) OAuthProvider
