package user

import "context"

// Repository is the storage port for the users table. Implementations must
// persist atomically. Emails are stored case-normalized (lowercase) but
// callers may pass any case to lookups.
type Repository interface {
	List(ctx context.Context) ([]User, error)
	Get(ctx context.Context, email string) (*User, error)
	Add(ctx context.Context, u User) error
	Remove(ctx context.Context, email string) error
	SetRole(ctx context.Context, email string, role Role) error
	Count(ctx context.Context) (int, error)
}
