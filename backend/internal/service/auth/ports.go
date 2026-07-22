package auth

import "context"

type OAuthConfigStore interface {
	OAuthConfig(context.Context) (OAuthConfig, error)
	SaveOAuthConfig(context.Context, OAuthConfig) error
}

type LocalAdminStore interface {
	LocalAdmin(context.Context) (*LocalAdminCredential, error)
	CreateLocalAdmin(context.Context, LocalAdminCredential) error
}

type Store interface {
	OAuthConfigStore
	LocalAdminStore
	SessionKey(context.Context) ([]byte, error)
}

type OAuthProvider interface {
	AuthCodeURL(state string) string
	ExchangeUser(ctx context.Context, code string) (User, error)
}

type OAuthProviderFactory func(clientID, clientSecret, redirectURL string) OAuthProvider
