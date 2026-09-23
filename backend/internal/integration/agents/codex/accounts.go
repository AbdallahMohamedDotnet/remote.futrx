package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

const accountValidationTimeout = 30 * time.Second

type accountLoginAttempt struct {
	accountID string
	label     string
	root      string
	home      string
	// external holds host credential files a Codex build that ignores the
	// isolated CODEX_HOME writes instead, such as a Snap package that pins
	// CODEX_HOME to its own data directory.
	external []watchedCredential
}

// watchedCredential remembers a host credential file as it was before an
// account login so the login's write can be detected and undone.
type watchedCredential struct {
	path     string
	previous []byte
	existed  bool
}

func watchCredential(path string) (watchedCredential, error) {
	previous, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return watchedCredential{}, err
	}
	return watchedCredential{path: path, previous: previous, existed: err == nil}, nil
}

// changed returns the file's current content when the login wrote to it.
func (w watchedCredential) changed() ([]byte, bool) {
	current, err := os.ReadFile(w.path)
	if err != nil {
		return nil, false
	}
	return current, !w.existed || !bytes.Equal(current, w.previous)
}

func (w watchedCredential) restore() error {
	if w.existed {
		return agentauth.WriteCredentialFile(w.path, w.previous)
	}
	err := os.Remove(w.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
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
	credential, err := os.ReadFile(codexCredentialPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("Codex is not signed in")
		}
		return fmt.Errorf("read current Codex credential: %w", err)
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

func (a *Auth) StartAccountLogin(ctx context.Context, label, accountID string) (agentauth.LoginSnapshot, error) {
	// Reconnecting a saved account may omit the label to keep its own.
	label, err := agentauth.NormalizeLoginLabel(label, accountID)
	if err != nil {
		return agentauth.LoginSnapshot{}, err
	}
	if a.LoginState().Active {
		return agentauth.LoginSnapshot{}, errors.New("a Codex login is already in progress")
	}

	a.mutationMu.Lock()
	a.mu.Lock()
	if a.activeRuns > 0 {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, agentauth.ErrAccountInUse
	}
	if a.attempt != nil {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, errors.New("a Codex account login is already in progress")
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
	root, home, err := prepareIsolatedCodexHome("remote-codex-login-*")
	if err != nil {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, fmt.Errorf("prepare Codex login: %w", err)
	}
	external := make([]watchedCredential, 0, 2)
	for _, path := range externalCredentialPaths() {
		watched, err := watchCredential(path)
		if err != nil {
			_ = os.RemoveAll(root)
			a.mu.Unlock()
			a.mutationMu.Unlock()
			return agentauth.LoginSnapshot{}, fmt.Errorf("read current Codex credential: %w", err)
		}
		external = append(external, watched)
	}
	a.attempt = &accountLoginAttempt{
		accountID: accountID, label: label, root: root, home: home,
		external: external,
	}
	a.mu.Unlock()
	a.mutationMu.Unlock()

	state, err := a.device.StartDeviceLogin(ctx)
	if err != nil {
		a.clearAccountAttempt()
	}
	return agentauth.LoginSnapshot{
		Active: state.Active, URL: state.VerificationURI, UserCode: state.UserCode,
		StartedAt: state.StartedAt, ExpiresAt: state.ExpiresAt,
		Completed: state.Completed, Error: state.Error,
	}, err
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
	if err := a.store.SaveAgentAccounts(ctx, agent.ProviderCodex, next); err != nil {
		a.mu.Unlock()
		return err
	}
	a.accounts = next
	a.mu.Unlock()
	a.Broadcast()
	return nil
}

func (a *Auth) BeginRun() (func(), error) {
	a.mutationMu.Lock()
	a.mu.Lock()
	if a.attempt != nil {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return nil, errors.New("Codex account login is in progress")
	}
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

func (a *Auth) CaptureActiveCredential(ctx context.Context) error {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	if a.accounts.ActiveAccountID == "" || a.store == nil {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()
	credential, err := os.ReadFile(codexCredentialPath())
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.accounts.Clone()
	for index := range next.Accounts {
		if next.Accounts[index].ID == next.ActiveAccountID {
			next.Accounts[index].Credential = append(json.RawMessage(nil), credential...)
			break
		}
	}
	if err := a.store.SaveAgentAccounts(ctx, agent.ProviderCodex, next); err != nil {
		return err
	}
	a.accounts = next
	return nil
}

func (a *Auth) deviceAuthEnv(base []string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.attempt != nil {
		return isolatedCodexAuthEnvFor(base, a.attempt.home)
	}
	return codexAuthEnv(base)
}

func (a *Auth) hasAccountAttempt() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.attempt != nil
}

func (a *Auth) completeAccountLogin(commandErr error, output string) agentauth.DeviceCompletion {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	attempt := a.attempt
	a.attempt = nil
	a.mu.Unlock()
	if attempt == nil {
		return agentauth.DeviceCompletion{Error: "Codex account login state was lost"}
	}
	defer os.RemoveAll(attempt.root)

	// Take the login's credential, then put every host file the CLI touched
	// back as it was. Saving the account below decides what becomes active.
	credential, readErr := attempt.loginCredential()
	if err := attempt.restoreExternal(); err != nil {
		return agentauth.DeviceCompletion{Error: "restore previous Codex credential: " + truncate(err.Error(), 160)}
	}
	if commandErr != nil {
		return accountLoginFailure(fmt.Sprintf("codex login failed: %s", truncate(commandErr.Error(), 300)), output)
	}
	if readErr != nil {
		return accountLoginFailure(truncate(readErr.Error(), 400), output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), accountValidationTimeout)
	defer cancel()
	validated, err := a.validate(ctx, credential)
	if err != nil {
		return agentauth.DeviceCompletion{Error: truncate(err.Error(), 300)}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.accounts.Clone()
	id := attempt.accountID
	if id == "" {
		id, err = agentauth.NewAccountID()
		if err != nil {
			return agentauth.DeviceCompletion{Error: "could not create an account identifier"}
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
			return agentauth.DeviceCompletion{Error: agentauth.ErrAccountNotFound.Error()}
		}
	}
	activate := a.activeRuns == 0
	if activate {
		next.ActiveAccountID = id
	}
	if err := a.persistLocked(ctx, next, func() json.RawMessage {
		if activate {
			return validated.Credential
		}
		return nil
	}()); err != nil {
		return agentauth.DeviceCompletion{Error: truncate(err.Error(), 300)}
	}
	a.accounts = next
	return agentauth.DeviceCompletion{Completed: true}
}

// loginCredential returns what the login wrote: the isolated CODEX_HOME
// file when the CLI honored it, otherwise a host file it changed instead.
func (attempt *accountLoginAttempt) loginCredential() ([]byte, error) {
	isolatedPath := filepath.Join(attempt.home, "auth.json")
	credential, err := os.ReadFile(isolatedPath)
	if err == nil {
		return credential, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read Codex login credential: %w", err)
	}
	checked := []string{isolatedPath}
	for _, watched := range attempt.external {
		if current, changed := watched.changed(); changed {
			return current, nil
		}
		checked = append(checked, watched.path)
	}
	return nil, fmt.Errorf("Codex login completed without writing credentials (checked %s)", strings.Join(checked, ", "))
}

func (attempt *accountLoginAttempt) restoreExternal() error {
	var failures []error
	for _, watched := range attempt.external {
		if _, changed := watched.changed(); !changed {
			continue
		}
		if err := watched.restore(); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// accountLoginFailure keeps the CLI's own last words, which usually say why
// no credential was written.
func accountLoginFailure(message, output string) agentauth.DeviceCompletion {
	if output != "" {
		message += "; codex output: " + output
	}
	return agentauth.DeviceCompletion{Error: message}
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

func prepareIsolatedCodexHome(pattern string) (string, string, error) {
	root, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", "", err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return "", "", err
	}
	home := filepath.Join(root, ".codex")
	if err := os.Mkdir(home, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return "", "", err
	}
	return root, home, nil
}

type validatedAccount struct {
	Email      string
	PlanType   string
	Credential json.RawMessage
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
	if err := a.store.SaveAgentAccounts(ctx, agent.ProviderCodex, next); err != nil {
		return err
	}
	if len(activeCredential) == 0 {
		return nil
	}
	if err := writeActiveCredential(activeCredential); err != nil {
		_ = a.store.SaveAgentAccounts(context.Background(), agent.ProviderCodex, previous)
		return err
	}
	return nil
}

func writeActiveCredential(credential []byte) error {
	return agentauth.WriteCredentialFile(codexCredentialPath(), credential)
}
