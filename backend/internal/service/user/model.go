package user

import "strings"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func NormalizeRole(r string) Role {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case string(RoleAdmin):
		return RoleAdmin
	case string(RoleMember), "":
		return RoleMember
	default:
		return ""
	}
}

func ValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleMember:
		return true
	}
	return false
}

// NormalizeEmail lowercases + trims. Used everywhere to compare emails
// case-insensitively; OAuth gives us lowercase but humans typing into the
// admin UI may not.
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// User is one registered email. AddedAt is unix millis; AddedBy is the
// email of the admin who added them (empty for the bootstrap admin).
type User struct {
	Email   string `json:"email"`
	Role    Role   `json:"role"`
	AddedAt int64  `json:"addedAt"`
	AddedBy string `json:"addedBy,omitempty"`
}
