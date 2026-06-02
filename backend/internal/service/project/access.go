package project

import "context"

// AccessRepository is the storage port for per-project membership lists. Each
// project gets a flat sorted list of registered emails. Emails are stored
// lowercased; lookups are case-insensitive.
type AccessRepository interface {
	List(ctx context.Context, projectID ID) ([]string, error)
	Add(ctx context.Context, projectID ID, email string) error
	Remove(ctx context.Context, projectID ID, email string) error
	Set(ctx context.Context, projectID ID, emails []string) error
	Has(ctx context.Context, projectID ID, email string) (bool, error)
	DeleteAll(ctx context.Context, projectID ID) error
}
