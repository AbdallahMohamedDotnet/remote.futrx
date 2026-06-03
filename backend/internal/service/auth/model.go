package auth

import "errors"

const (
	SessionCookieName = "remote_session"
	StateCookieName   = "remote_oauth_state"
)

var ErrOAuthConfigNotFound = errors.New("oauth config not found")

type OAuthConfig struct {
	GoogleClientID     string `json:"googleClientId"`
	GoogleClientSecret string `json:"googleClientSecret"`
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
	Authenticated bool   `json:"authenticated"`
	Claimed       bool   `json:"claimed"`
	AdminEmail    string `json:"adminEmail,omitempty"`
	Email         string `json:"email,omitempty"`
	Sub           string `json:"sub,omitempty"`
	IsAdmin       bool   `json:"isAdmin,omitempty"`
	IsRegistered  bool   `json:"isRegistered,omitempty"`
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
