package auth

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

var (
	ErrAccountLabelRequired = errors.New("account label is required")
	ErrAccountLabelInvalid  = errors.New("account label is invalid")
	ErrAccountLabelConflict = errors.New("account label already exists")
	ErrAccountNotFound      = errors.New("agent account not found")
	ErrAccountInUse         = errors.New("cannot switch accounts while the agent is running")
	ErrActiveAccountDelete  = errors.New("the active account cannot be removed")
)

// AccountRecord is the private persistence model for one provider credential.
// Credential must never be serialized through an HTTP response.
type AccountRecord struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	Email       string          `json:"email,omitempty"`
	PlanType    string          `json:"planType,omitempty"`
	ValidatedAt time.Time       `json:"validatedAt"`
	Credential  json.RawMessage `json:"credential"`
}

type AccountSet struct {
	ActiveAccountID string          `json:"activeAccountId,omitempty"`
	Accounts        []AccountRecord `json:"accounts"`
}

// AccountStore persists opaque provider credentials. Provider integrations
// remain responsible for interpreting and validating Credential.
type AccountStore interface {
	AgentAccounts(context.Context, agent.ProviderID) (AccountSet, error)
	SaveAgentAccounts(context.Context, agent.ProviderID, AccountSet) error
}

// Account is the redacted account metadata exposed to clients.
type Account struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Email       string     `json:"email,omitempty"`
	PlanType    string     `json:"planType,omitempty"`
	ValidatedAt *time.Time `json:"validatedAt,omitempty"`
	Active      bool       `json:"active"`
}

type AccountsSnapshot struct {
	ActiveAccountID string    `json:"activeAccountId,omitempty"`
	Items           []Account `json:"items"`
}

// AccountController is the optional multi-account capability attached to a
// provider auth binding. Transport can expose it without knowing credential
// formats or provider login policy.
type AccountController interface {
	AccountsSnapshot() AccountsSnapshot
	ImportCurrent(context.Context, string) error
	StartAccountLogin(context.Context, string, string) (LoginSnapshot, error)
	ActivateAccount(context.Context, string) error
	DeleteAccount(context.Context, string) error
}

// Clone returns a deep copy so callers can stage a mutation without sharing
// credential bytes with the committed set.
func (s AccountSet) Clone() AccountSet {
	clone := AccountSet{ActiveAccountID: s.ActiveAccountID}
	clone.Accounts = make([]AccountRecord, len(s.Accounts))
	copy(clone.Accounts, s.Accounts)
	for index := range clone.Accounts {
		clone.Accounts[index].Credential = append(json.RawMessage(nil), s.Accounts[index].Credential...)
	}
	return clone
}

// Find returns the saved account with id.
func (s AccountSet) Find(id string) (AccountRecord, bool) {
	for _, record := range s.Accounts {
		if record.ID == id {
			return record, true
		}
	}
	return AccountRecord{}, false
}

// Snapshot redacts credentials into the client-facing account list.
func (s AccountSet) Snapshot() AccountsSnapshot {
	snapshot := AccountsSnapshot{
		ActiveAccountID: s.ActiveAccountID,
		Items:           make([]Account, 0, len(s.Accounts)),
	}
	for _, record := range s.Accounts {
		var validatedAt *time.Time
		if !record.ValidatedAt.IsZero() {
			value := record.ValidatedAt
			validatedAt = &value
		}
		snapshot.Items = append(snapshot.Items, Account{
			ID: record.ID, Label: record.Label, Email: record.Email,
			PlanType: record.PlanType, ValidatedAt: validatedAt,
			Active: record.ID == s.ActiveAccountID,
		})
	}
	return snapshot
}

// EnsureUniqueLabel rejects a case-insensitive label collision with any
// account other than exceptID.
func (s AccountSet) EnsureUniqueLabel(label, exceptID string) error {
	for _, record := range s.Accounts {
		if record.ID != exceptID && strings.EqualFold(record.Label, label) {
			return fmt.Errorf("%w: an account named %q already exists", ErrAccountLabelConflict, label)
		}
	}
	return nil
}

// NormalizeAccountLabel trims a user-supplied label and enforces its bounds.
func NormalizeAccountLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", ErrAccountLabelRequired
	}
	if len([]rune(label)) > 64 {
		return "", fmt.Errorf("%w: account label must be 64 characters or fewer", ErrAccountLabelInvalid)
	}
	return label, nil
}

// NewAccountID returns a random opaque account identifier.
func NewAccountID() (string, error) {
	value := make([]byte, 16)
	if _, err := crand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

// WriteCredentialFile atomically replaces path with a mode-0600 file so a
// concurrent CLI never observes a partially written credential.
func WriteCredentialFile(path string, credential []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".credential-*.tmp")
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
