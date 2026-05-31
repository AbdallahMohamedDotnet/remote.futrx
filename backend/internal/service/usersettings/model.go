package usersettings

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrNotFound        = errors.New("user settings not found")
	ErrInvalidIdentity = errors.New("user settings identity is required")
	ErrInvalidTheme    = errors.New("invalid appearance theme")
	ErrInvalidEnvKey   = errors.New("invalid env-var key")
)

type Key string

const LocalAdminKey Key = "local-admin"

type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeDark   Theme = "dark"
	ThemeLight  Theme = "light"
)

// envKeyRe is the standard env-var name shape: upper/digit/underscore, must
// not start with a digit. Anything else gets rejected so we can never push
// `lxc config set environment.foo bar` with malformed keys.
var envKeyRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

type Settings struct {
	Appearance Appearance        `json:"appearance"`
	// Secrets is the global env-var bag pushed into every project container.
	// Keys are upper-snake env-var names (e.g. CLOUDFLARE_API_TOKEN); values
	// are the plaintext secrets. Stored on disk in mode 0o700 root-only
	// data/user-settings/sha256-<hash>.json. Never returned plaintext over
	// HTTP — the handler swaps each value for "set" on GET.
	Secrets   map[string]string `json:"secrets,omitempty"`
	UpdatedAt int64             `json:"updatedAt,omitempty"`
}

type Appearance struct {
	Theme Theme `json:"theme"`
}

// SecretsUpdate is the partial-update payload for the secrets bag.
//   - Set inserts or replaces values.
//   - Unset removes keys.
// A key appearing in both Set and Unset is treated as a Set (set wins).
type SecretsUpdate struct {
	Set   map[string]string `json:"set,omitempty"`
	Unset []string          `json:"unset,omitempty"`
}

type UpdateInput struct {
	Appearance *AppearanceUpdate `json:"appearance,omitempty"`
	Secrets    *SecretsUpdate    `json:"secrets,omitempty"`
}

type AppearanceUpdate struct {
	Theme *Theme `json:"theme,omitempty"`
}

// SettingsDiff captures what actually changed during an Update so the caller
// can propagate just the delta to side-effect targets (e.g. push to every
// project container).
type SettingsDiff struct {
	SecretsSet   map[string]string
	SecretsUnset []string
}

// Empty reports whether the diff carries nothing to apply.
func (d SettingsDiff) Empty() bool {
	return len(d.SecretsSet) == 0 && len(d.SecretsUnset) == 0
}

func DefaultSettings() Settings {
	return Settings{
		Appearance: Appearance{Theme: ThemeSystem},
	}
}

func ValidTheme(theme Theme) bool {
	switch theme {
	case ThemeSystem, ThemeDark, ThemeLight:
		return true
	default:
		return false
	}
}

// ValidEnvKey reports whether key is a legal env-var name (UPPER_SNAKE).
func ValidEnvKey(key string) bool {
	return envKeyRe.MatchString(key)
}

func validateSecretsUpdate(u *SecretsUpdate) error {
	if u == nil {
		return nil
	}
	for k := range u.Set {
		if !ValidEnvKey(k) {
			return fmt.Errorf("%w: %q (must match %s)", ErrInvalidEnvKey, k, envKeyRe.String())
		}
	}
	for _, k := range u.Unset {
		if !ValidEnvKey(k) {
			return fmt.Errorf("%w: %q (must match %s)", ErrInvalidEnvKey, k, envKeyRe.String())
		}
	}
	return nil
}
