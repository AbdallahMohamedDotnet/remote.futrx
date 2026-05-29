package auth

import "context"

type Store interface {
	Admin(ctx context.Context) (*Admin, error)
	SaveAdmin(ctx context.Context, admin Admin) error
}

type OAuthProvider interface {
	AuthCodeURL(state string) string
	ExchangeUser(ctx context.Context, code string) (User, error)
}
