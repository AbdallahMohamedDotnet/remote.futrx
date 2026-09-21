package codex

import (
	"bytes"
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

func TestReadCodexAccountRefreshesAndReturnsPlanMetadata(t *testing.T) {
	stdout := strings.NewReader(
		`{"id":1,"result":{}}` + "\n" +
			`{"id":2,"result":{"account":{"type":"chatgpt","email":"person@example.test","planType":"pro"},"requiresOpenaiAuth":true}}` + "\n",
	)
	var stdin bytes.Buffer
	account, err := readCodexAccountRPC(&stdin, stdout)
	if err != nil {
		t.Fatal(err)
	}
	if account.Account == nil || account.Account.Email != "person@example.test" || account.Account.PlanType != "pro" {
		t.Fatalf("account = %#v", account)
	}
	if !strings.Contains(stdin.String(), `"method":"account/read"`) || !strings.Contains(stdin.String(), `"refreshToken":true`) {
		t.Fatalf("requests = %s", stdin.String())
	}
}

func TestSnapCredentialPathUsesSnapManagedCodexHome(t *testing.T) {
	got, ok := snapCredentialPathFor("/snap/bin/codex", "/home/person")
	if !ok || got != "/home/person/snap/codex/current/auth.json" {
		t.Fatalf("snap credential path = %q, %v", got, ok)
	}
	if _, ok := snapCredentialPathFor("/usr/local/bin/codex", "/home/person"); ok {
		t.Fatal("non-Snap Codex was detected as a Snap command")
	}
}

func TestAccountLoginCapturesCredentialWrittenThroughHome(t *testing.T) {
	binDir := t.TempDir()
	script := filepath.Join(binDir, "codex")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
mkdir -p "$HOME/.codex"
printf '%s' '{"auth_mode":"chatgpt","token":"new"}' > "$HOME/.codex/auth.json"
printf '%s\n' 'https://auth.openai.com/codex/device'
printf '%s\n' 'ABCD-12345'
`), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", filepath.Join(t.TempDir(), "host-home"))
	canonicalHome := filepath.Join(t.TempDir(), ".codex")
	t.Setenv("CODEX_HOME", canonicalHome)

	store := &memoryAccountStore{}
	auth, err := NewAuth(store)
	if err != nil {
		t.Fatal(err)
	}
	auth.validate = func(_ context.Context, credential []byte) (validatedAccount, error) {
		return validatedAccount{
			Email: "new@example.test", PlanType: "plus",
			Credential: append(json.RawMessage(nil), credential...),
		}, nil
	}
	if _, err := auth.StartAccountLogin(context.Background(), "Company", ""); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for auth.LoginState().Active && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	state := auth.LoginState()
	if !state.Completed || state.Error != "" {
		t.Fatalf("login state = %#v", state)
	}
	if len(store.accounts.Accounts) != 1 || store.accounts.ActiveAccountID == "" {
		t.Fatalf("saved accounts = %#v", store.accounts)
	}
	written, err := os.ReadFile(filepath.Join(canonicalHome, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(store.accounts.Accounts[0].Credential) {
		t.Fatalf("active credential = %s, saved credential = %s", written, store.accounts.Accounts[0].Credential)
	}
}

func TestCompleteAccountLoginCapturesExternalCredentialPath(t *testing.T) {
	canonicalHome := filepath.Join(t.TempDir(), ".codex")
	t.Setenv("CODEX_HOME", canonicalHome)
	externalPath := filepath.Join(t.TempDir(), "snap", "codex", "current", "auth.json")
	newCredential := json.RawMessage(`{"auth_mode":"chatgpt","token":"new"}`)
	if err := agentauth.WriteCredentialFile(externalPath, newCredential); err != nil {
		t.Fatal(err)
	}
	store := &memoryAccountStore{}
	auth, err := NewAuth(store)
	if err != nil {
		t.Fatal(err)
	}
	auth.validate = func(_ context.Context, credential []byte) (validatedAccount, error) {
		return validatedAccount{Email: "company@example.test", PlanType: "plus", Credential: credential}, nil
	}
	root, home, err := prepareIsolatedCodexHome("remote-codex-test-*")
	if err != nil {
		t.Fatal(err)
	}
	auth.attempt = &accountLoginAttempt{
		label: "Company", root: root, home: home, credentialPath: externalPath,
	}
	completion := auth.completeAccountLogin(nil)
	if !completion.Completed || completion.Error != "" {
		t.Fatalf("completion = %#v", completion)
	}
	if len(store.accounts.Accounts) != 1 || string(store.accounts.Accounts[0].Credential) != string(newCredential) {
		t.Fatalf("saved accounts = %#v", store.accounts)
	}
	active, err := os.ReadFile(filepath.Join(canonicalHome, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != string(newCredential) {
		t.Fatalf("active credential = %s", active)
	}
}

func TestFailedExternalAccountLoginRestoresPreviousCredential(t *testing.T) {
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), ".codex"))
	externalPath := filepath.Join(t.TempDir(), "snap", "codex", "current", "auth.json")
	previous := json.RawMessage(`{"auth_mode":"chatgpt","token":"old"}`)
	if err := agentauth.WriteCredentialFile(externalPath, []byte(`{"auth_mode":"chatgpt","token":"new"}`)); err != nil {
		t.Fatal(err)
	}
	auth, err := NewAuth(&memoryAccountStore{})
	if err != nil {
		t.Fatal(err)
	}
	auth.validate = func(context.Context, []byte) (validatedAccount, error) {
		return validatedAccount{}, errors.New("invalid account")
	}
	root, home, err := prepareIsolatedCodexHome("remote-codex-test-*")
	if err != nil {
		t.Fatal(err)
	}
	auth.attempt = &accountLoginAttempt{
		label: "Company", root: root, home: home, credentialPath: externalPath,
		previousCredential: previous, hadPrevious: true,
	}
	completion := auth.completeAccountLogin(nil)
	if completion.Error != "invalid account" {
		t.Fatalf("completion = %#v", completion)
	}
	restored, err := os.ReadFile(externalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(previous) {
		t.Fatalf("restored credential = %s", restored)
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

func TestActivateAccountValidatesBeforeReplacingCurrentCredential(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	oldCredential := json.RawMessage(`{"auth_mode":"chatgpt","token":"old"}`)
	newCredential := json.RawMessage(`{"auth_mode":"chatgpt","token":"new"}`)
	if err := os.WriteFile(filepath.Join(home, "auth.json"), oldCredential, 0o600); err != nil {
		t.Fatal(err)
	}
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
		return validatedAccount{Email: "new@example.test", PlanType: "pro", Credential: credential}, nil
	}
	if err := auth.ActivateAccount(context.Background(), "new"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newCredential) {
		t.Fatalf("active credential = %s", got)
	}
	snapshot := auth.AccountsSnapshot()
	if snapshot.ActiveAccountID != "new" || !snapshot.Items[1].Active || snapshot.Items[1].PlanType != "pro" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "token") {
		t.Fatalf("credential leaked through account snapshot: %s", encoded)
	}
}

func TestActivateAccountFailureAndRunLeasePreserveCurrentCredential(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	oldCredential := json.RawMessage(`{"auth_mode":"chatgpt","token":"old"}`)
	newCredential := json.RawMessage(`{"auth_mode":"chatgpt","token":"new"}`)
	if err := os.WriteFile(filepath.Join(home, "auth.json"), oldCredential, 0o600); err != nil {
		t.Fatal(err)
	}
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
	got, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(oldCredential) || auth.AccountsSnapshot().ActiveAccountID != "old" {
		t.Fatalf("failed switch changed active account: credential=%s snapshot=%#v", got, auth.AccountsSnapshot())
	}
}

func TestAccountLabelsAreBoundedAndUnique(t *testing.T) {
	accounts := agentauth.AccountSet{Accounts: []agentauth.AccountRecord{{ID: "one", Label: "Personal"}}}
	if err := accounts.EnsureUniqueLabel("personal", ""); !errors.Is(err, agentauth.ErrAccountLabelConflict) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := agentauth.NormalizeAccountLabel(strings.Repeat("x", 65)); !errors.Is(err, agentauth.ErrAccountLabelInvalid) {
		t.Fatalf("long label error = %v", err)
	}
	if snapshot := (agentauth.AccountSet{Accounts: []agentauth.AccountRecord{{
		ID: "one", Label: "Personal", ValidatedAt: time.Now(), Credential: json.RawMessage(`{"secret":true}`),
	}}}).Snapshot(); len(snapshot.Items) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
