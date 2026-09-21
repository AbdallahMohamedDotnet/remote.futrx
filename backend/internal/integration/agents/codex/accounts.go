package codex

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
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
	accountID          string
	label              string
	root               string
	home               string
	credentialPath     string
	previousCredential json.RawMessage
	hadPrevious        bool
}

func (a *Auth) AccountsSnapshot() agentauth.AccountsSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return accountSnapshot(a.accounts)
}

func accountSnapshot(accounts agentauth.AccountSet) agentauth.AccountsSnapshot {
	snapshot := agentauth.AccountsSnapshot{
		ActiveAccountID: accounts.ActiveAccountID,
		Items:           make([]agentauth.Account, 0, len(accounts.Accounts)),
	}
	for _, record := range accounts.Accounts {
		var validatedAt *time.Time
		if !record.ValidatedAt.IsZero() {
			value := record.ValidatedAt
			validatedAt = &value
		}
		snapshot.Items = append(snapshot.Items, agentauth.Account{
			ID: record.ID, Label: record.Label, Email: record.Email,
			PlanType: record.PlanType, ValidatedAt: validatedAt,
			Active: record.ID == accounts.ActiveAccountID,
		})
	}
	return snapshot
}

func (a *Auth) ImportCurrent(ctx context.Context, label string) error {
	label, err := normalizeAccountLabel(label)
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
	validated, err := a.validate(ctx, credential)
	if err != nil {
		return err
	}

	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	a.mu.Lock()
	if err := uniqueAccountLabel(a.accounts, label, ""); err != nil {
		a.mu.Unlock()
		return err
	}
	id, err := newAccountID()
	if err != nil {
		a.mu.Unlock()
		return err
	}
	next := cloneAccounts(a.accounts)
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
	label, err := normalizeAccountLabel(label)
	if err != nil {
		return agentauth.LoginSnapshot{}, err
	}
	if a.LoginState().Active {
		return agentauth.LoginSnapshot{}, errors.New("a Codex login is already in progress")
	}

	a.mutationMu.Lock()
	a.mu.Lock()
	if a.attempt != nil {
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, errors.New("a Codex account login is already in progress")
	}
	if accountID != "" {
		record, ok := findAccount(a.accounts, accountID)
		if !ok {
			a.mu.Unlock()
			a.mutationMu.Unlock()
			return agentauth.LoginSnapshot{}, agentauth.ErrAccountNotFound
		}
		if label == "" {
			label = record.Label
		}
	}
	if err := uniqueAccountLabel(a.accounts, label, accountID); err != nil {
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
	credentialPath := filepath.Join(home, "auth.json")
	if snapPath, ok := snapCodexCredentialPath(); ok {
		credentialPath = snapPath
	}
	previousCredential, readErr := os.ReadFile(credentialPath)
	hadPrevious := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		_ = os.RemoveAll(root)
		a.mu.Unlock()
		a.mutationMu.Unlock()
		return agentauth.LoginSnapshot{}, fmt.Errorf("read current Codex credential: %w", readErr)
	}
	a.attempt = &accountLoginAttempt{
		accountID: accountID, label: label, root: root, home: home,
		credentialPath: credentialPath, previousCredential: previousCredential,
		hadPrevious: hadPrevious,
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
	record, ok := findAccount(a.accounts, accountID)
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
	next := cloneAccounts(a.accounts)
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
	next := cloneAccounts(a.accounts)
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

func (a *Auth) BeginRun() func() {
	a.mu.Lock()
	a.activeRuns++
	a.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			a.activeRuns--
			a.mu.Unlock()
		})
	}
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
	next := cloneAccounts(a.accounts)
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

func (a *Auth) completeAccountLogin(commandErr error) agentauth.DeviceCompletion {
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
	if commandErr != nil {
		return a.failedAccountLogin(attempt, fmt.Sprintf("codex login failed: %s", truncate(commandErr.Error(), 300)))
	}
	credential, err := os.ReadFile(attempt.credentialPath)
	if err != nil {
		return a.failedAccountLogin(attempt, "Codex login completed without writing credentials")
	}
	if attempt.hadPrevious && bytes.Equal(credential, attempt.previousCredential) {
		return a.failedAccountLogin(attempt, "Codex login completed without updating credentials")
	}
	ctx, cancel := context.WithTimeout(context.Background(), accountValidationTimeout)
	defer cancel()
	validated, err := a.validate(ctx, credential)
	if err != nil {
		return a.failedAccountLogin(attempt, truncate(err.Error(), 300))
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	next := cloneAccounts(a.accounts)
	id := attempt.accountID
	if id == "" {
		id, err = newAccountID()
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
		return a.failedAccountLogin(attempt, truncate(err.Error(), 300))
	}
	a.accounts = next
	return agentauth.DeviceCompletion{Completed: true}
}

func (a *Auth) failedAccountLogin(attempt *accountLoginAttempt, message string) agentauth.DeviceCompletion {
	isolatedPath := filepath.Join(attempt.home, "auth.json")
	if attempt.credentialPath == isolatedPath {
		return agentauth.DeviceCompletion{Error: message}
	}
	var err error
	if attempt.hadPrevious {
		err = writeCredentialFile(attempt.credentialPath, attempt.previousCredential)
	} else {
		err = os.Remove(attempt.credentialPath)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	}
	if err != nil {
		message += "; restore previous credential: " + truncate(err.Error(), 160)
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
	previous := cloneAccounts(a.accounts)
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
	return writeCredentialFile(codexCredentialPath(), credential)
}

func writeCredentialFile(path string, credential []byte) error {
	home := filepath.Dir(path)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(home, ".auth-*.json.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(credential); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func normalizeAccountLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", agentauth.ErrAccountLabelRequired
	}
	if len([]rune(label)) > 64 {
		return "", fmt.Errorf("%w: account label must be 64 characters or fewer", agentauth.ErrAccountLabelInvalid)
	}
	return label, nil
}

func uniqueAccountLabel(accounts agentauth.AccountSet, label, exceptID string) error {
	for _, record := range accounts.Accounts {
		if record.ID != exceptID && strings.EqualFold(record.Label, label) {
			return fmt.Errorf("%w: an account named %q already exists", agentauth.ErrAccountLabelConflict, label)
		}
	}
	return nil
}

func findAccount(accounts agentauth.AccountSet, id string) (agentauth.AccountRecord, bool) {
	for _, record := range accounts.Accounts {
		if record.ID == id {
			return record, true
		}
	}
	return agentauth.AccountRecord{}, false
}

func cloneAccounts(accounts agentauth.AccountSet) agentauth.AccountSet {
	clone := agentauth.AccountSet{ActiveAccountID: accounts.ActiveAccountID}
	clone.Accounts = make([]agentauth.AccountRecord, len(accounts.Accounts))
	copy(clone.Accounts, accounts.Accounts)
	for index := range clone.Accounts {
		clone.Accounts[index].Credential = append(json.RawMessage(nil), accounts.Accounts[index].Credential...)
	}
	return clone
}

func newAccountID() (string, error) {
	value := make([]byte, 16)
	if _, err := crand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
