package claude

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

func TestParseClaudeAuthStatusReadsSubscriptionMetadata(t *testing.T) {
	status, err := parseClaudeAuthStatus([]byte("notice\n" +
		`{"loggedIn":true,"authMethod":"claude.ai","email":"person@example.test","subscriptionType":"max"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !status.LoggedIn || status.AuthMethod != "claude.ai" || status.Email != "person@example.test" || status.SubscriptionType != "max" {
		t.Fatalf("status = %#v", status)
	}
}

func TestCheckOAuthUsableRejectsExpiredLoginWithoutRefreshToken(t *testing.T) {
	now := time.UnixMilli(2_000_000)
	for _, test := range []struct {
		oauth string
		ok    bool
	}{
		{oauth: `{"refreshToken":"r","expiresAt":1}`, ok: true},
		{oauth: `{"accessToken":"a","expiresAt":3000000}`, ok: true},
		{oauth: `{"accessToken":"a","expiresAt":1000000}`, ok: false},
		{oauth: `{}`, ok: false},
	} {
		if err := checkOAuthUsable(json.RawMessage(test.oauth), now); (err == nil) != test.ok {
			t.Fatalf("checkOAuthUsable(%s) = %v, want ok=%v", test.oauth, err, test.ok)
		}
	}
}

func TestIsolatedClaudeAuthEnvOverridesInheritedCredentials(t *testing.T) {
	env := isolatedClaudeAuthEnvFor([]string{
		"HOME=/root",
		"CLAUDE_CONFIG_DIR=/root/.claude",
		"ANTHROPIC_API_KEY=sk-test",
		"CLAUDE_CODE_OAUTH_TOKEN=token",
	}, "/tmp/isolated-claude")
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "CLAUDE_CONFIG_DIR=/root/.claude") || !strings.Contains(joined, "CLAUDE_CONFIG_DIR=/tmp/isolated-claude") {
		t.Fatalf("isolated auth env = %#v", env)
	}
	if strings.Contains(joined, "ANTHROPIC_API_KEY=") || strings.Contains(joined, "CLAUDE_CODE_OAUTH_TOKEN=") {
		t.Fatalf("inherited credential leaked into isolated auth env: %#v", env)
	}
}

func TestValidateAccountCredentialKeepsTokensRefreshedByStatus(t *testing.T) {
	installFakeClaude(t, `
if [ "$1 $2" = "auth status" ]; then
  grep -q '"refreshToken": "old"' "$CLAUDE_CONFIG_DIR/.credentials.json" || exit 1
  printf '%s' '{"claudeAiOauth":{"refreshToken":"rotated"}}' > "$CLAUDE_CONFIG_DIR/.credentials.json"
  printf '%s\n' '{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"pro"}'
fi
`)
	validated, err := validateAccountCredential(context.Background(), testClaudeCredential("old", "uuid-1", "person@example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if validated.Email != "person@example.test" || validated.PlanType != "pro" {
		t.Fatalf("validated = %#v", validated)
	}
	if !strings.Contains(string(validated.Credential), "rotated") || !strings.Contains(string(validated.Credential), "uuid-1") {
		t.Fatalf("validated credential = %s", validated.Credential)
	}
}

func TestValidateAccountCredentialRejectsSignedOutStatus(t *testing.T) {
	installFakeClaude(t, `
printf '%s\n' '{"loggedIn":false,"authMethod":"none"}'
exit 1
`)
	if _, err := validateAccountCredential(context.Background(), testClaudeCredential("old", "uuid-1", "")); err == nil {
		t.Fatal("signed-out credential was accepted")
	}
}

func TestAccountLoginCapturesCredentialWrittenToIsolatedConfig(t *testing.T) {
	installFakeClaude(t, `
if [ "$1 $2" = "auth login" ]; then
  printf '%s\n' 'https://claude.com/cai/oauth/authorize?code=true&state=test'
  read code
  printf '%s' '{"claudeAiOauth":{"refreshToken":"new"}}' > "$CLAUDE_CONFIG_DIR/.credentials.json"
  printf '%s' '{"oauthAccount":{"accountUuid":"uuid-new","emailAddress":"new@example.test"}}' > "$CLAUDE_CONFIG_DIR/.claude.json"
fi
`)
	host := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", host)
	writeHostFiles(t, host,
		`{"claudeAiOauth":{"refreshToken":"old"},"mcpOAuth":{"server":"kept"}}`,
		`{"oauthAccount":{"accountUuid":"uuid-old"},"projects":{"/workspace":{}}}`,
	)

	store := &memoryAccountStore{}
	auth, err := NewAuth(store)
	if err != nil {
		t.Fatal(err)
	}
	auth.validate = acceptCredential
	login, err := auth.StartAccountLogin(context.Background(), "Company", "")
	if err != nil {
		t.Fatal(err)
	}
	if !login.Active || !login.AwaitingCode || !strings.HasPrefix(login.URL, "https://claude.com/cai/oauth/authorize") {
		t.Fatalf("login = %#v", login)
	}
	if err := auth.SubmitCode(context.Background(), "pasted-code"); err != nil {
		t.Fatal(err)
	}
	if state := auth.Status().Login; !state.Completed || state.Error != "" {
		t.Fatalf("login state = %#v", state)
	}
	if len(store.accounts.Accounts) != 1 || store.accounts.ActiveAccountID != store.accounts.Accounts[0].ID {
		t.Fatalf("saved accounts = %#v", store.accounts)
	}
	credentials := readFile(t, filepath.Join(host, ".credentials.json"))
	config := readFile(t, filepath.Join(host, ".claude.json"))
	if !strings.Contains(credentials, `"new"`) || !strings.Contains(credentials, "kept") {
		t.Fatalf("host credentials = %s", credentials)
	}
	if !strings.Contains(config, "uuid-new") || !strings.Contains(config, "/workspace") {
		t.Fatalf("host config = %s", config)
	}
}

func TestActivateAccountValidatesBeforeReplacingCurrentCredential(t *testing.T) {
	host := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", host)
	oldCredential := testClaudeCredential("old", "uuid-old", "old@example.test")
	newCredential := testClaudeCredential("new", "uuid-new", "new@example.test")
	writeHostFiles(t, host,
		`{"claudeAiOauth":{"refreshToken":"old"},"mcpOAuth":{"server":"kept"}}`,
		`{"oauthAccount":{"accountUuid":"uuid-old"},"projects":{"/workspace":{}}}`,
	)
	store := &memoryAccountStore{accounts: agentauth.AccountSet{
		ActiveAccountID: "old",
		Accounts: []agentauth.AccountRecord{
			{ID: "old", Label: "Old", Credential: oldCredential},
			{ID: "new", Label: "New", Credential: newCredential},
		},
	}}
	auth, err := NewAuth(store)
	if err != nil {
		t.Fatal(err)
	}
	auth.validate = func(_ context.Context, credential []byte) (validatedAccount, error) {
		return validatedAccount{Email: "new@example.test", PlanType: "max", Credential: credential}, nil
	}
	if err := auth.ActivateAccount(context.Background(), "new"); err != nil {
		t.Fatal(err)
	}
	credentials := readFile(t, filepath.Join(host, ".credentials.json"))
	config := readFile(t, filepath.Join(host, ".claude.json"))
	if !strings.Contains(credentials, `"new"`) || !strings.Contains(credentials, "kept") {
		t.Fatalf("host credentials = %s", credentials)
	}
	if !strings.Contains(config, "uuid-new") || !strings.Contains(config, "/workspace") {
		t.Fatalf("host config = %s", config)
	}
	snapshot := auth.AccountsSnapshot()
	if snapshot.ActiveAccountID != "new" || !snapshot.Items[1].Active || snapshot.Items[1].PlanType != "max" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "refreshToken") {
		t.Fatalf("credential leaked through account snapshot: %s", encoded)
	}
}

func TestActivateAccountFailureAndRunLeasePreserveCurrentCredential(t *testing.T) {
	host := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", host)
	writeHostFiles(t, host, `{"claudeAiOauth":{"refreshToken":"old"}}`, `{"oauthAccount":{"accountUuid":"uuid-old"}}`)
	store := &memoryAccountStore{accounts: agentauth.AccountSet{
		ActiveAccountID: "old",
		Accounts: []agentauth.AccountRecord{
			{ID: "old", Label: "Old", Credential: testClaudeCredential("old", "uuid-old", "")},
			{ID: "new", Label: "New", Credential: testClaudeCredential("new", "uuid-new", "")},
		},
	}}
	auth, err := NewAuth(store)
	if err != nil {
		t.Fatal(err)
	}
	auth.validate = acceptCredential
	release, err := auth.BeginRun()
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ActivateAccount(context.Background(), "new"); !errors.Is(err, agentauth.ErrAccountInUse) {
		t.Fatalf("activate during run error = %v", err)
	}
	release()
	auth.validate = func(context.Context, []byte) (validatedAccount, error) {
		return validatedAccount{}, errors.New("expired credential")
	}
	if err := auth.ActivateAccount(context.Background(), "new"); err == nil || err.Error() != "expired credential" {
		t.Fatalf("invalid activation error = %v", err)
	}
	if got := readFile(t, filepath.Join(host, ".credentials.json")); !strings.Contains(got, `"old"`) || auth.AccountsSnapshot().ActiveAccountID != "old" {
		t.Fatalf("failed switch changed active account: credential=%s snapshot=%#v", got, auth.AccountsSnapshot())
	}
}

func TestNewAuthKeepsNewerHostTokensForTheSameActiveAccount(t *testing.T) {
	host := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", host)
	writeHostFiles(t, host, `{"claudeAiOauth":{"refreshToken":"refreshed"}}`, `{"oauthAccount":{"accountUuid":"uuid-1"}}`)
	store := &memoryAccountStore{accounts: agentauth.AccountSet{
		ActiveAccountID: "one",
		Accounts:        []agentauth.AccountRecord{{ID: "one", Label: "One", Credential: testClaudeCredential("stale", "uuid-1", "")}},
	}}
	if _, err := NewAuth(store); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(host, ".credentials.json")); !strings.Contains(got, "refreshed") {
		t.Fatalf("host credential was overwritten: %s", got)
	}
	if saved := string(store.accounts.Accounts[0].Credential); !strings.Contains(saved, "refreshed") {
		t.Fatalf("saved credential = %s", saved)
	}
}

func TestNewAuthRestoresActiveAccountOverDifferentHostLogin(t *testing.T) {
	host := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", host)
	writeHostFiles(t, host, `{"claudeAiOauth":{"refreshToken":"other"}}`, `{"oauthAccount":{"accountUuid":"uuid-other"}}`)
	store := &memoryAccountStore{accounts: agentauth.AccountSet{
		ActiveAccountID: "one",
		Accounts:        []agentauth.AccountRecord{{ID: "one", Label: "One", Credential: testClaudeCredential("saved", "uuid-1", "")}},
	}}
	if _, err := NewAuth(store); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(host, ".credentials.json")); !strings.Contains(got, "saved") {
		t.Fatalf("active account was not restored: %s", got)
	}
}

type memoryAccountStore struct{ accounts agentauth.AccountSet }

func (s *memoryAccountStore) AgentAccounts(context.Context, agent.ProviderID) (agentauth.AccountSet, error) {
	return s.accounts.Clone(), nil
}

func (s *memoryAccountStore) SaveAgentAccounts(_ context.Context, _ agent.ProviderID, accounts agentauth.AccountSet) error {
	s.accounts = accounts.Clone()
	return nil
}

func acceptCredential(_ context.Context, credential []byte) (validatedAccount, error) {
	return validatedAccount{Email: "person@example.test", PlanType: "pro", Credential: append(json.RawMessage(nil), credential...)}, nil
}

func testClaudeCredential(refreshToken, accountUUID, email string) json.RawMessage {
	credential, err := json.Marshal(accountCredential{
		Credentials: map[string]json.RawMessage{
			"claudeAiOauth": json.RawMessage(`{"refreshToken":"` + refreshToken + `"}`),
		},
		OAuthAccount: json.RawMessage(`{"accountUuid":"` + accountUUID + `","emailAddress":"` + email + `"}`),
	})
	if err != nil {
		panic(err)
	}
	return credential
}

func installFakeClaude(t *testing.T, body string) {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeHostFiles(t *testing.T, dir, credentials, config string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(credentials), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
