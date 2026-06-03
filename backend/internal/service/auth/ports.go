package auth

import "context"

type Store interface {
	// reserved for future per-server admin metadata
}

type OAuthProvider interface {
	AuthCodeURL(state string) string
	ExchangeUser(ctx context.Context, code string) (User, error)
}
