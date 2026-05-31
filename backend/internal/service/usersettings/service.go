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

func (s *Service) Update(ctx context.Context, key Key, input UpdateInput) (Settings, error) {
	settings, err := s.Get(ctx, key)
	if err != nil {
		return Settings{}, err
	}

	if input.Appearance != nil && input.Appearance.Theme != nil {
		theme := Theme(strings.TrimSpace(string(*input.Appearance.Theme)))
		if !ValidTheme(theme) {
			return Settings{}, ErrInvalidTheme
		}
		settings.Appearance.Theme = theme
	}

	settings.UpdatedAt = time.Now().UnixMilli()
	return s.repo.Save(ctx, key, settings)
}

func normalize(settings Settings) Settings {
	if !ValidTheme(settings.Appearance.Theme) {
		settings.Appearance.Theme = ThemeSystem
	}
	return settings
}
