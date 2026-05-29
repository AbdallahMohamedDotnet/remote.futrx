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

type Admin struct {
	Email     string `json:"email"`
	Sub       string `json:"sub"`
	Name      string `json:"name,omitempty"`
	Picture   string `json:"picture,omitempty"`
	ClaimedAt int64  `json:"claimedAt"`
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
}

type ClaimedError struct {
	Email string
}

func (e ClaimedError) Error() string {
	return "server is claimed by " + e.Email
}
