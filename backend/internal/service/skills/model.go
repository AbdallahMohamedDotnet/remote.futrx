package skills

import "errors"

var ErrInvalidProvider = errors.New("invalid skill provider")

type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
)

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Provider    Provider `json:"provider"`
	Source      string   `json:"source,omitempty"`
}
