package auth

import (
	"context"
	"encoding/json"
	"errors"
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
