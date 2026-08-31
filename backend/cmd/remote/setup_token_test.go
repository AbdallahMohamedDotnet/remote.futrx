package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serviceauth "github.com/futrx-com/remote.futrx.com/internal/service/auth"
	"github.com/futrx-com/remote.futrx.com/internal/stores/fileauth"
)

func TestSetupTokenCommandPrintsAFragmentURLAndStoresOnlyTheHash(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	if err := runSetupToken(context.Background(), dir, "https://remote.example.com/", &out); err != nil {
		t.Fatalf("runSetupToken: %v", err)
	}

	printed := out.String()
	// A fragment never reaches the server, so the token cannot land in a
	// proxy access log the way a query string would.
	if !strings.Contains(printed, "https://remote.example.com/#token=") {
		t.Fatalf("printed output did not carry a fragment token URL:\n%s", printed)
	}
	if strings.Contains(printed, "?token=") {
		t.Fatalf("token was printed as a query string, which proxies log:\n%s", printed)
	}

	record, err := fileauth.New(dir).SetupToken(context.Background())
	if err != nil || record == nil {
		t.Fatalf("SetupToken record = %#v, %v", record, err)
	}
	token := strings.TrimSpace(strings.SplitN(strings.SplitN(printed, "#token=", 2)[1], "\n", 2)[0])
	if token == "" {
		t.Fatal("could not read the printed token back")
	}
	if record.Hash == token {
		t.Fatal("the plaintext token was persisted; only its hash may be stored")
	}
	if !strings.Contains(printed, token) {
		t.Fatal("printed token did not round-trip")
	}
}

// Reissuing against a configured server would print a setup URL that cannot
// work, since the claim is refused as already-claimed regardless.
func TestSetupTokenCommandRefusesOnceClaimed(t *testing.T) {
	dir := t.TempDir()
	credential, err := json.Marshal(serviceauth.LocalAdminCredential{
		Email: "admin@example.com", PasswordHash: "$argon2id$hash",
	})
	if err != nil {
		t.Fatalf("marshal credential: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local-admin.json"), credential, 0o600); err != nil {
		t.Fatalf("seed local-admin.json: %v", err)
	}

	var out bytes.Buffer
	if err := runSetupToken(context.Background(), dir, "https://remote.example.com", &out); err == nil {
		t.Fatal("runSetupToken succeeded against an already-configured server")
	}
	if out.Len() != 0 {
		t.Fatalf("refused command still printed: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "setup-token.json")); !os.IsNotExist(err) {
		t.Fatalf("refused command still wrote a token record (stat err = %v)", err)
	}
}
