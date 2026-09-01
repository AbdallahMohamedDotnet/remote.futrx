package fileauth

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
)

type Store struct {
	dataDir string
	mu      sync.Mutex
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

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.oauthConfigLocked()
}

func (s *Store) oauthConfigLocked() (serviceauth.OAuthConfig, error) {
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

func (s *Store) SaveOAuthConfig(ctx context.Context, cfg serviceauth.OAuthConfig) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	cfg.GoogleClientID = strings.TrimSpace(cfg.GoogleClientID)
	cfg.GoogleClientSecret = strings.TrimSpace(cfg.GoogleClientSecret)
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		return serviceauth.ErrInvalidOAuthConfig
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSONLocked("oauth.json", cfg)
}

func (s *Store) LocalAdmin(ctx context.Context) (*serviceauth.LocalAdminCredential, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.localAdminLocked()
}

func (s *Store) localAdminLocked() (*serviceauth.LocalAdminCredential, error) {
	data, err := os.ReadFile(filepath.Join(s.dataDir, "local-admin.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read local-admin.json: %w", err)
	}
	var credential serviceauth.LocalAdminCredential
	if err := json.Unmarshal(data, &credential); err != nil {
		return nil, fmt.Errorf("parse local-admin.json: %w", err)
	}
	credential.Email = strings.ToLower(strings.TrimSpace(credential.Email))
	if credential.Email == "" || credential.PasswordHash == "" {
		return nil, errors.New("local-admin.json is incomplete")
	}
	return &credential, nil
}

func (s *Store) CreateLocalAdmin(ctx context.Context, credential serviceauth.LocalAdminCredential) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	credential.Email = strings.ToLower(strings.TrimSpace(credential.Email))
	if credential.Email == "" || credential.PasswordHash == "" {
		return errors.New("local admin credential is incomplete")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(filepath.Join(s.dataDir, "local-admin.json")); err == nil {
		return serviceauth.ErrLocalAdminAlreadyClaimed
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.writeJSONLocked("local-admin.json", credential)
}

func (s *Store) DeleteLocalAdmin(ctx context.Context, expected serviceauth.LocalAdminCredential) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	expected.Email = strings.ToLower(strings.TrimSpace(expected.Email))
	if expected.Email == "" || expected.PasswordHash == "" {
		return errors.New("expected local admin credential is incomplete")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.localAdminLocked()
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if current.Email != expected.Email || current.PasswordHash != expected.PasswordHash {
		return serviceauth.ErrLocalAdminCredentialChanged
	}
	if err := os.Remove(filepath.Join(s.dataDir, "local-admin.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete local-admin.json: %w", err)
	}
	return nil
}

func (s *Store) SetupToken(ctx context.Context) (*serviceauth.SetupTokenRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(s.dataDir, "setup-token.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read setup-token.json: %w", err)
	}
	var record serviceauth.SetupTokenRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse setup-token.json: %w", err)
	}
	if record.Hash == "" {
		return nil, errors.New("setup-token.json is incomplete")
	}
	return &record, nil
}

// SaveSetupToken overwrites any existing record, which is what makes a
// restart or a CLI reissue rotate the token rather than accumulate several
// live ones.
func (s *Store) SaveSetupToken(ctx context.Context, record serviceauth.SetupTokenRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if record.Hash == "" {
		return errors.New("setup token record is incomplete")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSONLocked("setup-token.json", record)
}

// DeleteSetupToken removes setup-token.json entirely, as opposed to
// SaveSetupToken which only overwrites/rotates the record. Currently unused
// in production code: SetupTokenGuard.Consume marks the record Used via
// SaveSetupToken instead of deleting it, so this method is only exercised by
// tests today. Kept on the SetupTokenStore port for a future caller (e.g.
// Consume switching to a hard delete on claim) — flag for review.
func (s *Store) DeleteSetupToken(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(filepath.Join(s.dataDir, "setup-token.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete setup-token.json: %w", err)
	}
	return nil
}

func (s *Store) SessionKey(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *Store) writeJSONLocked(name string, value any) error {
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dataDir, ".auth-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(s.dataDir, name))
}
