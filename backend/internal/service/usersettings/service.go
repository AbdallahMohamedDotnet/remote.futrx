package usersettings

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func KeyFromSession(email, sub string) (Key, error) {
	sub = strings.TrimSpace(sub)
	if sub != "" {
		return Key("sub:" + sub), nil
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email != "" {
		return Key("email:" + email), nil
	}

	return "", ErrInvalidIdentity
}

func (s *Service) Get(ctx context.Context, key Key) (Settings, error) {
	if strings.TrimSpace(string(key)) == "" {
		return Settings{}, ErrInvalidIdentity
	}

	settings, err := s.repo.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return DefaultSettings(), nil
		}
		return Settings{}, err
	}
	return normalize(settings), nil
}

// Update applies the partial input to the stored settings and returns:
//   - the new full settings,
//   - a SettingsDiff describing what actually changed (so callers can
//     propagate just the delta — e.g. push the new secrets to every
//     container without reading every container's current env),
//   - any persistence/validation error.
//
// Idempotent: re-applying the same input is a no-op with an empty diff.
func (s *Service) Update(ctx context.Context, key Key, input UpdateInput) (Settings, SettingsDiff, error) {
	settings, err := s.Get(ctx, key)
	if err != nil {
		return Settings{}, SettingsDiff{}, err
	}

	if input.Appearance != nil && input.Appearance.Theme != nil {
		theme := Theme(strings.TrimSpace(string(*input.Appearance.Theme)))
		if !ValidTheme(theme) {
			return Settings{}, SettingsDiff{}, ErrInvalidTheme
		}
		settings.Appearance.Theme = theme
	}

	var diff SettingsDiff
	if input.Secrets != nil {
		if err := validateSecretsUpdate(input.Secrets); err != nil {
			return Settings{}, SettingsDiff{}, err
		}
		if settings.Secrets == nil {
			settings.Secrets = map[string]string{}
		}
		for k, v := range input.Secrets.Set {
			if prior, ok := settings.Secrets[k]; ok && prior == v {
				continue
			}
			if diff.SecretsSet == nil {
				diff.SecretsSet = map[string]string{}
			}
			diff.SecretsSet[k] = v
			settings.Secrets[k] = v
		}
		for _, k := range input.Secrets.Unset {
			if _, ok := input.Secrets.Set[k]; ok {
				// Set wins over Unset for the same key.
				continue
			}
			if _, ok := settings.Secrets[k]; ok {
				diff.SecretsUnset = append(diff.SecretsUnset, k)
				delete(settings.Secrets, k)
			}
		}
		if len(settings.Secrets) == 0 {
			settings.Secrets = nil
		}
	}

	settings.UpdatedAt = time.Now().UnixMilli()
	saved, err := s.repo.Save(ctx, key, settings)
	if err != nil {
		return Settings{}, SettingsDiff{}, err
	}
	return saved, diff, nil
}

func normalize(settings Settings) Settings {
	if !ValidTheme(settings.Appearance.Theme) {
		settings.Appearance.Theme = ThemeSystem
	}
	return settings
}
