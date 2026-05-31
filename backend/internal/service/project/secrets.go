package project

import "context"

// Secret is one per-project environment variable. Stored on the host and
// injected as `--env KEY=VALUE` when running provider commands inside the
// project's container. Never synced into the container's filesystem.
type Secret struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updatedAt"`
}

// SecretsRepository is the storage port for project secrets. Implementations
// must persist atomically; concurrent callers from different goroutines are
// serialized.
type SecretsRepository interface {
	List(ctx context.Context, projectID ID) ([]Secret, error)
	Set(ctx context.Context, projectID ID, key, value string) (Secret, error)
	Delete(ctx context.Context, projectID ID, key string) error
	DeleteAll(ctx context.Context, projectID ID) error
}

// ValidSecretKey allows POSIX-style env var names: [A-Za-z_][A-Za-z0-9_]*.
// We're strict here so injection into `lxc exec --env KEY=VAL` is always
// safe — no spaces, no '=', no control characters.
func ValidSecretKey(k string) bool {
	if k == "" || len(k) > 128 {
		return false
	}
	for i, r := range k {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
