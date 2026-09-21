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

type memoryAccountStore struct{ accounts agentauth.AccountSet }

func (s *memoryAccountStore) AgentAccounts(context.Context, agent.ProviderID) (agentauth.AccountSet, error) {
	return cloneAccounts(s.accounts), nil
}

func (s *memoryAccountStore) SaveAgentAccounts(_ context.Context, _ agent.ProviderID, accounts agentauth.AccountSet) error {
	s.accounts = cloneAccounts(accounts)
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
	release := auth.BeginRun()
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
	if err := uniqueAccountLabel(accounts, "personal", ""); !errors.Is(err, agentauth.ErrAccountLabelConflict) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := normalizeAccountLabel(strings.Repeat("x", 65)); !errors.Is(err, agentauth.ErrAccountLabelInvalid) {
		t.Fatalf("long label error = %v", err)
	}
	if snapshot := accountSnapshot(agentauth.AccountSet{Accounts: []agentauth.AccountRecord{{
		ID: "one", Label: "Personal", ValidatedAt: time.Now(), Credential: json.RawMessage(`{"secret":true}`),
	}}}); len(snapshot.Items) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
