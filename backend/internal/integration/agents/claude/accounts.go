package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

const accountValidationTimeout = 30 * time.Second

// accountCredentialKeys are the .credentials.json entries that belong to one
// Claude subscription login. Other entries, such as MCP server OAuth tokens,
// stay with the host and survive an account switch.
var accountCredentialKeys = []string{"claudeAiOauth", "organizationUuid"}

// accountCredential is the opaque value saved per account: the account's
// token entries plus the oauthAccount profile the CLI keeps in .claude.json.
type accountCredential struct {
	Credentials  map[string]json.RawMessage `json:"credentials"`
	OAuthAccount json.RawMessage            `json:"oauthAccount,omitempty"`
}

type accountLoginAttempt struct {
	accountID string
	label     string
	root      string
	home      string
}

type validatedAccount struct {
	Email      string
	PlanType   string
	Credential json.RawMessage
}

func (a *Auth) AccountsSnapshot() agentauth.AccountsSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.accounts.Snapshot()
}

func (a *Auth) ImportCurrent(ctx context.Context, label string) error {
	label, err := agentauth.NormalizeAccountLabel(label)
	if err != nil {
		return err
	}
	credential, err := readAccountCredential(claudeCredentialPath(), claudeGlobalConfigPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("Claude is not signed in")
		}
		return fmt.Errorf("read current Claude credential: %w", err)
	}
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	if a.activeRuns > 0 {
		a.mu.Unlock()
		return agentauth.ErrAccountInUse
	}
	a.mu.Unlock()
	validated, err := a.validate(ctx, credential)
	if err != nil {
		return err
	}

	a.mu.Lock()
	if err := a.accounts.EnsureUniqueLabel(label, ""); err != nil {
		a.mu.Unlock()
		return err
	}
	id, err := agentauth.NewAccountID()
	if err != nil {
		a.mu.Unlock()
		return err
	}
	next := a.accounts.Clone()
	next.Accounts = append(next.Accounts, accountRecord(id, label, validated))
	activeCredential := json.RawMessage(nil)
	if a.activeRuns == 0 {
		next.ActiveAccountID = id
		activeCredential = validated.Credential
	}
	if err := a.persistLocked(ctx, next, activeCredential); err != nil {
		a.mu.Unlock()
		return err
	}
	a.accounts = next
	a.mu.Unlock()
	a.Broadcast()
	return nil
}

// StartAccountLogin runs `claude auth login` against a private config
// directory. The pasted code is submitted through the regular code route;
// resolveCompletion then validates and saves what the CLI wrote there.
func (a *Auth) StartAccountLogin(ctx context.Context, label, accountID string) (agentauth.LoginSnapshot, error) {
	label, err := agentauth.NormalizeAccountLabel(label)
	if err != nil {
		return agentauth.LoginSnapshot{}, err
	}
	// A code login that was abandoned or timed out never resolves on its
	// own, so a new account login replaces it instead of waiting for it.
	if err := a.code.Cancel(ctx); err != nil {
		return agentauth.LoginSnapshot{}, err
	}

	a.mutationMu.Lock()
	a.mu.Lock()
	if a.activeRuns > 0 {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, agentauth.ErrAccountInUse
	}
	stale := a.attempt
	a.attempt = nil
	if stale != nil {
		_ = os.RemoveAll(stale.root)
	}
	if accountID != "" {
		record, ok := a.accounts.Find(accountID)
		if !ok {
			a.mu.Unlock()
			a.mutationMu.Unlock()
			return agentauth.LoginSnapshot{}, agentauth.ErrAccountNotFound
		}
		if label == "" {
			label = record.Label
		}
	}
	if err := a.accounts.EnsureUniqueLabel(label, accountID); err != nil {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, err
	}
	root, home, err := prepareIsolatedClaudeHome("remote-claude-login-*")
	if err != nil {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, fmt.Errorf("prepare Claude login: %w", err)
	}
	a.attempt = &accountLoginAttempt{accountID: accountID, label: label, root: root, home: home}
	a.mu.Unlock()
	a.mutationMu.Unlock()

	if _, err := a.code.Start(ctx); err != nil {
		a.clearAccountAttempt()
		return agentauth.LoginSnapshot{}, err
	}
	state := a.code.Status().Login
	return agentauth.LoginSnapshot{
		Active: state.Active, URL: state.AuthURL, AwaitingCode: state.AwaitingCode,
		StartedAt: state.StartedAt, Completed: state.Completed, Error: state.Error,
	}, nil
}

func (a *Auth) ActivateAccount(ctx context.Context, accountID string) error {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	a.mu.Lock()
	if a.activeRuns > 0 {
		a.mu.Unlock()
		return agentauth.ErrAccountInUse
	}
	record, ok := a.accounts.Find(accountID)
	a.mu.Unlock()
	if !ok {
		return agentauth.ErrAccountNotFound
	}
	validated, err := a.validate(ctx, record.Credential)
	if err != nil {
		return err
	}

	a.mu.Lock()
	if a.activeRuns > 0 {
		a.mu.Unlock()
		return agentauth.ErrAccountInUse
	}
	next := a.accounts.Clone()
	for index := range next.Accounts {
		if next.Accounts[index].ID == accountID {
			next.Accounts[index] = accountRecord(accountID, next.Accounts[index].Label, validated)
			break
		}
	}
	next.ActiveAccountID = accountID
	if err := a.persistLocked(ctx, next, validated.Credential); err != nil {
		a.mu.Unlock()
		return err
	}
	a.accounts = next
	a.mu.Unlock()
	a.Broadcast()
	return nil
}

func (a *Auth) DeleteAccount(ctx context.Context, accountID string) error {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	if accountID == a.accounts.ActiveAccountID {
		a.mu.Unlock()
		return agentauth.ErrActiveAccountDelete
	}
	next := a.accounts.Clone()
	found := false
	filtered := next.Accounts[:0]
	for _, record := range next.Accounts {
		if record.ID == accountID {
			found = true
			continue
		}
		filtered = append(filtered, record)
	}
	if !found {
		a.mu.Unlock()
		return agentauth.ErrAccountNotFound
	}
	next.Accounts = filtered
	if a.store == nil {
		a.mu.Unlock()
		return errors.New("agent account store is unavailable")
	}
	if err := a.store.SaveAgentAccounts(ctx, agent.ProviderClaude, next); err != nil {
		a.mu.Unlock()
		return err
	}
	a.accounts = next
	a.mu.Unlock()
	a.Broadcast()
	return nil
}

// BeginRun leases the active account for one Claude run so it cannot be
// switched underneath the CLI. Unlike Codex, a pending account login does not
// block runs: Claude logins always write to a private CLAUDE_CONFIG_DIR, and
// completion activates the new account only when no run holds a lease.
func (a *Auth) BeginRun() (func(), error) {
	a.mutationMu.Lock()
	a.mu.Lock()
	a.activeRuns++
	a.mu.Unlock()
	a.mutationMu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			a.activeRuns--
			a.mu.Unlock()
		})
	}, nil
}

// CaptureActiveCredential saves tokens the CLI refreshed during a run back
// into the active account so a later switch does not restore stale tokens.
func (a *Auth) CaptureActiveCredential(ctx context.Context) error {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	if a.accounts.ActiveAccountID == "" || a.store == nil {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()
	credential, err := readAccountCredential(claudeCredentialPath(), claudeGlobalConfigPath())
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.accounts.Clone()
	for index := range next.Accounts {
		if next.Accounts[index].ID == next.ActiveAccountID {
			next.Accounts[index].Credential = credential
			break
		}
	}
	if err := a.store.SaveAgentAccounts(ctx, agent.ProviderClaude, next); err != nil {
		return err
	}
	a.accounts = next
	return nil
}

// restoreActiveAccount makes the saved active account the host login at
// startup. When the host already holds that same account, its tokens may be
// newer than the saved copy (capability probes also run the CLI), so the
// saved copy is refreshed from the host instead of overwriting it.
func (a *Auth) restoreActiveAccount() error {
	active, ok := a.accounts.Find(a.accounts.ActiveAccountID)
	if !ok {
		return nil
	}
	current, err := readAccountCredential(claudeCredentialPath(), claudeGlobalConfigPath())
	if err == nil && sameAccount(current, active.Credential) {
		for index := range a.accounts.Accounts {
			if a.accounts.Accounts[index].ID == active.ID {
				a.accounts.Accounts[index].Credential = current
			}
		}
		return a.store.SaveAgentAccounts(context.Background(), agent.ProviderClaude, a.accounts)
	}
	return writeActiveCredential(active.Credential)
}

func (a *Auth) codeAuthEnv(base []string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.attempt != nil {
		return isolatedClaudeAuthEnvFor(base, a.attempt.home)
	}
	return base
}

// resolveCompletion claims the result of a code login that was started for a
// saved account. Plain logins keep CodeService's default credential check.
func (a *Auth) resolveCompletion(exitErr error) (bool, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	attempt := a.attempt
	a.attempt = nil
	a.mu.Unlock()
	if attempt == nil {
		return false, nil
	}
	defer os.RemoveAll(attempt.root)
	if exitErr != nil && !errors.Is(exitErr, context.Canceled) {
		return true, fmt.Errorf("claude login failed: %s", truncate(exitErr.Error(), 300))
	}
	credential, err := readAccountCredential(
		filepath.Join(attempt.home, ".credentials.json"),
		filepath.Join(attempt.home, ".claude.json"),
	)
	if err != nil {
		return true, errors.New("Claude login completed without writing credentials")
	}
	ctx, cancel := context.WithTimeout(context.Background(), accountValidationTimeout)
	defer cancel()
	validated, err := a.validate(ctx, credential)
	if err != nil {
		return true, errors.New(truncate(err.Error(), 300))
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.accounts.Clone()
	id := attempt.accountID
	if id == "" {
		id, err = agentauth.NewAccountID()
		if err != nil {
			return true, errors.New("could not create an account identifier")
		}
		next.Accounts = append(next.Accounts, accountRecord(id, attempt.label, validated))
	} else {
		updated := false
		for index := range next.Accounts {
			if next.Accounts[index].ID == id {
				next.Accounts[index] = accountRecord(id, attempt.label, validated)
				updated = true
				break
			}
		}
		if !updated {
			return true, agentauth.ErrAccountNotFound
		}
	}
	activeCredential := json.RawMessage(nil)
	if a.activeRuns == 0 {
		next.ActiveAccountID = id
		activeCredential = validated.Credential
	}
	if err := a.persistLocked(ctx, next, activeCredential); err != nil {
		return true, errors.New(truncate(err.Error(), 300))
	}
	a.accounts = next
	return true, nil
}

func (a *Auth) clearAccountAttempt() {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	attempt := a.attempt
	a.attempt = nil
	a.mu.Unlock()
	if attempt != nil {
		_ = os.RemoveAll(attempt.root)
	}
}

func prepareIsolatedClaudeHome(pattern string) (string, string, error) {
	root, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", "", err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return "", "", err
	}
	home := filepath.Join(root, ".claude")
	if err := os.Mkdir(home, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return "", "", err
	}
	return root, home, nil
}

func accountRecord(id, label string, validated validatedAccount) agentauth.AccountRecord {
	return agentauth.AccountRecord{
		ID: id, Label: label, Email: validated.Email, PlanType: validated.PlanType,
		ValidatedAt: time.Now().UTC(),
		Credential:  append(json.RawMessage(nil), validated.Credential...),
	}
}

func (a *Auth) persistLocked(ctx context.Context, next agentauth.AccountSet, activeCredential json.RawMessage) error {
	if a.store == nil {
		return errors.New("agent account store is unavailable")
	}
	previous := a.accounts.Clone()
	if err := a.store.SaveAgentAccounts(ctx, agent.ProviderClaude, next); err != nil {
		return err
	}
	if len(activeCredential) == 0 {
		return nil
	}
	if err := writeActiveCredential(activeCredential); err != nil {
		_ = a.store.SaveAgentAccounts(context.Background(), agent.ProviderClaude, previous)
		return err
	}
	return nil
}

func writeActiveCredential(credential []byte) error {
	return writeAccountCredential(claudeCredentialPath(), claudeGlobalConfigPath(), credential)
}

// readAccountCredential extracts one account's entries from the CLI's
// credential and global config files.
func readAccountCredential(credentialPath, configPath string) (json.RawMessage, error) {
	credentials, err := readJSONObject(credentialPath)
	if err != nil {
		return nil, err
	}
	if len(credentials["claudeAiOauth"]) == 0 {
		return nil, fmt.Errorf("%s has no Claude subscription login: %w", credentialPath, os.ErrNotExist)
	}
	stored := accountCredential{Credentials: map[string]json.RawMessage{}}
	for _, key := range accountCredentialKeys {
		if value, ok := credentials[key]; ok {
			stored.Credentials[key] = value
		}
	}
	config, err := readJSONObject(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	stored.OAuthAccount = config["oauthAccount"]
	return json.Marshal(stored)
}

// writeAccountCredential merges one account into the CLI's files, keeping
// every unrelated entry. If the second file cannot be written, the first is
// restored so the host never mixes two accounts.
func writeAccountCredential(credentialPath, configPath string, credential []byte) error {
	var stored accountCredential
	if err := json.Unmarshal(credential, &stored); err != nil || len(stored.Credentials["claudeAiOauth"]) == 0 {
		return errors.New("saved Claude credential is not a Claude subscription login")
	}
	previousCredentials, readErr := os.ReadFile(credentialPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	credentials, err := readJSONObject(credentialPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if credentials == nil {
		credentials = map[string]json.RawMessage{}
	}
	for _, key := range accountCredentialKeys {
		delete(credentials, key)
		if value, ok := stored.Credentials[key]; ok {
			credentials[key] = value
		}
	}
	config, err := readJSONObject(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if config == nil {
		config = map[string]json.RawMessage{}
	}
	delete(config, "oauthAccount")
	if len(stored.OAuthAccount) > 0 {
		config["oauthAccount"] = stored.OAuthAccount
	}

	if err := writeJSONObject(credentialPath, credentials); err != nil {
		return err
	}
	if err := writeJSONObject(configPath, config); err != nil {
		if readErr == nil {
			_ = agentauth.WriteCredentialFile(credentialPath, previousCredentials)
		} else {
			_ = os.Remove(credentialPath)
		}
		return err
	}
	return nil
}

// sameAccount reports whether two saved credentials belong to one Claude
// account, comparing the stable account UUID before falling back to email.
func sameAccount(left, right []byte) bool {
	leftID, rightID := accountIdentity(left), accountIdentity(right)
	return leftID != "" && leftID == rightID
}

func accountIdentity(credential []byte) string {
	var stored accountCredential
	if json.Unmarshal(credential, &stored) != nil || len(stored.OAuthAccount) == 0 {
		return ""
	}
	var profile struct {
		AccountUUID  string `json:"accountUuid"`
		EmailAddress string `json:"emailAddress"`
	}
	if json.Unmarshal(stored.OAuthAccount, &profile) != nil {
		return ""
	}
	if profile.AccountUUID != "" {
		return "uuid:" + profile.AccountUUID
	}
	if profile.EmailAddress != "" {
		return "email:" + profile.EmailAddress
	}
	return ""
}

func readJSONObject(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if object == nil {
		object = map[string]json.RawMessage{}
	}
	return object, nil
}

func writeJSONObject(path string, object map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return err
	}
	return agentauth.WriteCredentialFile(path, data)
}
