package fileauth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
)

func TestLocalAdminCredentialIsPrivateAndCreateOnly(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)
	credential := serviceauth.LocalAdminCredential{
		Email: "admin@example.com", PasswordHash: "$argon2id$hash",
	}
	if err := store.CreateLocalAdmin(context.Background(), credential); err != nil {
		t.Fatalf("CreateLocalAdmin: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "local-admin.json"))
	if err != nil {
		t.Fatalf("stat local-admin.json: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("local-admin.json mode = %o, want 600", got)
	}
	if err := store.CreateLocalAdmin(context.Background(), credential); !errors.Is(err, serviceauth.ErrLocalAdminAlreadyClaimed) {
		t.Fatalf("second CreateLocalAdmin error = %v", err)
	}
}

func TestOAuthSecretIsPrivate(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)
	if err := store.SaveOAuthConfig(context.Background(), serviceauth.OAuthConfig{
		GoogleClientID: "id", GoogleClientSecret: "secret",
	}); err != nil {
		t.Fatalf("SaveOAuthConfig: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "oauth.json"))
	if err != nil {
		t.Fatalf("stat oauth.json: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("oauth.json mode = %o, want 600", got)
	}
}
