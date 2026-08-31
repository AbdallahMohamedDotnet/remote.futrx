package auth

import "context"

type OAuthConfigStore interface {
	OAuthConfig(context.Context) (OAuthConfig, error)
	SaveOAuthConfig(context.Context, OAuthConfig) error
}

type LocalAdminStore interface {
	LocalAdmin(context.Context) (*LocalAdminCredential, error)
	CreateLocalAdmin(context.Context, LocalAdminCredential) error
	// DeleteLocalAdmin removes only the credential that exactly matches the
	// expected value. It is reserved for compensating a failed claim.
	DeleteLocalAdmin(context.Context, LocalAdminCredential) error
}

// SetupTokenStore persists the single first-boot setup token. SetupToken
// returns (nil, nil) when none has been issued - that absence is the correct
// "no setup pending" state, not an error.
type SetupTokenStore interface {
	SetupToken(context.Context) (*SetupTokenRecord, error)
	SaveSetupToken(context.Context, SetupTokenRecord) error
	DeleteSetupToken(context.Context) error
}

type Store interface {
	OAuthConfigStore
	LocalAdminStore
	SetupTokenStore
	SessionKey(context.Context) ([]byte, error)
}

type OAuthProvider interface {
	AuthCodeURL(state string) string
	ExchangeUser(ctx context.Context, code string) (User, error)
}

type OAuthProviderFactory func(clientID, clientSecret, redirectURL string) OAuthProvider
