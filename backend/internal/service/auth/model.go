package auth

import (
	"errors"
	"time"
)

const (
	SessionCookieName = "remote_session"
	StateCookieName   = "remote_oauth_state"
)

var ErrOAuthConfigNotFound = errors.New("oauth config not found")

var (
	ErrLocalAdminAlreadyClaimed    = errors.New("local admin is already configured")
	ErrLocalAdminCredentialChanged = errors.New("local admin credential no longer matches")
	ErrAdminClaimUnauthorized      = errors.New("an existing administrator must authorize local password setup")
	ErrInvalidCredentials          = errors.New("invalid email or password")
	ErrPasswordTooShort            = errors.New("password must be at least 12 characters")
	ErrPasswordTooLong             = errors.New("password is too long")
	ErrInvalidOAuthConfig          = errors.New("Google OAuth client ID and client secret are required")
	ErrGoogleOAuthDisabled         = errors.New("Google sign-in is not configured")
	ErrLocalAdminPasswordOnly      = errors.New("the local administrator must sign in with a password")
)

type OAuthConfig struct {
	GoogleClientID     string `json:"googleClientId"`
	GoogleClientSecret string `json:"googleClientSecret"`
}

type LocalAdminCredential struct {
	Email        string `json:"email"`
	PasswordHash string `json:"passwordHash"`
}

// SetupTokenRecord is the durable half of the first-boot setup token. Only
// the hash is persisted, so a leaked data directory yields nothing usable -
// the plaintext exists solely in the terminal output that printed it once.
// Used is set after a claim consumes the token, which is what makes a token
// single-use even before it expires.
type SetupTokenRecord struct {
	Hash      string    `json:"hash"`
	ExpiresAt time.Time `json:"expiresAt"`
	Used      bool      `json:"used"`
}

type User struct {
	Email   string
	Sub     string
	Name    string
	Picture string
}

type Session struct {
	Email string `json:"email"`
	Sub   string `json:"sub"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

type Status struct {
	Authenticated        bool   `json:"authenticated"`
	Claimed              bool   `json:"claimed"`
	LocalAdminConfigured bool   `json:"localAdminConfigured"`
	GoogleOAuthEnabled   bool   `json:"googleOAuthEnabled"`
	GoogleClientID       string `json:"googleClientId,omitempty"`
	AdminEmail           string `json:"adminEmail,omitempty"`
	Email                string `json:"email,omitempty"`
	Sub                  string `json:"sub,omitempty"`
	IsAdmin              bool   `json:"isAdmin,omitempty"`
	IsRegistered         bool   `json:"isRegistered,omitempty"`
}

// ClaimedError is returned in the legacy single-admin path when a second
// user tries to sign in before the users-store is wired up.

// NotInvitedError is returned by Login when a Google OAuth flow succeeded
// but the resulting email is not in the users store. Surfaced to the
// frontend so the login screen can show a friendly "ask an admin" message.
type NotInvitedError struct {
	Email string
}

func (e NotInvitedError) Error() string {
	return "not invited - ask an admin to add your email (" + e.Email + ")"
}
