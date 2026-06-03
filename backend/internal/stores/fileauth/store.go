package fileauth

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
)

type Store struct {
	dataDir string
}

func New(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

func (s *Store) OAuthConfig(ctx context.Context) (serviceauth.OAuthConfig, error) {
	select {
	case <-ctx.Done():
		return serviceauth.OAuthConfig{}, ctx.Err()
	default:
	}

	data, err := os.ReadFile(filepath.Join(s.dataDir, "oauth.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return serviceauth.OAuthConfig{}, serviceauth.ErrOAuthConfigNotFound
		}
		return serviceauth.OAuthConfig{}, fmt.Errorf("read oauth.json: %w", err)
	}
	var cfg serviceauth.OAuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return serviceauth.OAuthConfig{}, fmt.Errorf("parse oauth.json: %w", err)
	}
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		return serviceauth.OAuthConfig{}, errors.New("oauth.json present but missing googleClientId or googleClientSecret")
	}
	return cfg, nil
}

func (s *Store) SessionKey(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(s.dataDir, "session.key")
	sessionKey, err := os.ReadFile(keyPath)
	if err == nil {
		return sessionKey, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read session.key: %w", err)
	}

	sessionKey = make([]byte, 32)
	if _, err := crand.Read(sessionKey); err != nil {
		return nil, fmt.Errorf("gen session key: %w", err)
	}
	if err := os.WriteFile(keyPath, sessionKey, 0o600); err != nil {
		return nil, fmt.Errorf("write session.key: %w", err)
	}
	return sessionKey, nil
}

