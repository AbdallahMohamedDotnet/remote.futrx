package usersettings

import "errors"

var (
	ErrNotFound        = errors.New("user settings not found")
	ErrInvalidIdentity = errors.New("user settings identity is required")
	ErrInvalidTheme    = errors.New("invalid appearance theme")
)

type Key string

const LocalAdminKey Key = "local-admin"

type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeDark   Theme = "dark"
	ThemeLight  Theme = "light"
)

type Settings struct {
	Appearance Appearance `json:"appearance"`
	UpdatedAt  int64      `json:"updatedAt,omitempty"`
}

type Appearance struct {
	Theme Theme `json:"theme"`
}

type UpdateInput struct {
	Appearance *AppearanceUpdate `json:"appearance,omitempty"`
}

type AppearanceUpdate struct {
	Theme *Theme `json:"theme,omitempty"`
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
