package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ID string
type ProjectID string
type Provider string

const (
	ProviderClaude      Provider = "claude"
	ProviderCodex       Provider = "codex"
	ProviderKimi        Provider = "kimi"
	ProviderAntigravity Provider = "antigravity"
)

type Meta struct {
	ID                   ID         `json:"id"`
	Title                string     `json:"title"`
	Provider             Provider   `json:"provider,omitempty"`
	ClaudeSessionID      string     `json:"claudeSessionId,omitempty"`
	CodexSessionID       string     `json:"codexSessionId,omitempty"`
	KimiSessionID        string     `json:"kimiSessionId,omitempty"`
	AntigravitySessionID string     `json:"antigravitySessionId,omitempty"`
	TmuxSession          string     `json:"tmuxSession,omitempty"`
	Cwd                  string     `json:"cwd,omitempty"`
	CreatedAt            int64      `json:"createdAt"`
	LastMessageAt        int64      `json:"lastMessageAt"`
	LastReadAt           int64      `json:"lastReadAt,omitempty"`
	Running              bool       `json:"running,omitempty"`
	Model                string     `json:"model"`
	Mode                 string     `json:"mode"`
	ReasoningEffort      string     `json:"reasoningEffort"`
	ServiceTier          string     `json:"serviceTier"`
	ProjectID            ProjectID  `json:"projectId,omitempty"`
	ForkPending          bool       `json:"forkPending,omitempty"`
	SelectedSkills       []SkillRef `json:"selectedSkills,omitempty"`
}

type SkillRef struct {
	Name     string   `json:"name"`
	Command  string   `json:"command,omitempty"`
	Provider Provider `json:"provider,omitempty"`
	Source   string   `json:"source,omitempty"`
}

type Event struct {
	Seq                  int64           `json:"seq,omitempty"`
	T                    int64           `json:"t"`
	Type                 string          `json:"type"`
	Text                 string          `json:"text,omitempty"`
	MessageID            string          `json:"messageId,omitempty"`
	ID                   string          `json:"id,omitempty"`
	Name                 string          `json:"name,omitempty"`
	Input                json.RawMessage `json:"input,omitempty"`
	Output               string          `json:"output,omitempty"`
	IsError              bool            `json:"isError,omitempty"`
	ToolName             string          `json:"toolName,omitempty"`
	Subtype              string          `json:"subtype,omitempty"`
	Data                 json.RawMessage `json:"data,omitempty"`
	ClaudeSessionID      string          `json:"claudeSessionId,omitempty"`
	CodexSessionID       string          `json:"codexSessionId,omitempty"`
	KimiSessionID        string          `json:"kimiSessionId,omitempty"`
	AntigravitySessionID string          `json:"antigravitySessionId,omitempty"`
	Provider             Provider        `json:"provider,omitempty"`
	Usage                json.RawMessage `json:"usage,omitempty"`
	Message              string          `json:"message,omitempty"`
	Running              bool            `json:"running,omitempty"`
	// ScheduledTaskID marks events produced by a scheduled run rather than an
	// interactive one, so consumers can tell "your turn finished" from "a task
	// ran while you were away".
	ScheduledTaskID string `json:"scheduledTaskId,omitempty"`
}

type EventPageQuery struct {
	Limit     int
	BeforeSeq int64
}

type EventPage struct {
	Events     []Event `json:"events"`
	NextBefore int64   `json:"nextBefore,omitempty"`
	LastSeq    int64   `json:"lastSeq"`
	HasMore    bool    `json:"hasMore"`
}

type CreateInput struct {
	Title           string     `json:"title,omitempty"`
	TmuxSession     string     `json:"tmuxSession,omitempty"`
	Cwd             string     `json:"cwd,omitempty"`
	Provider        Provider   `json:"provider,omitempty"`
	Model           string     `json:"model,omitempty"`
	Mode            string     `json:"mode,omitempty"`
	ReasoningEffort string     `json:"reasoningEffort,omitempty"`
	ServiceTier     string     `json:"serviceTier,omitempty"`
	ProjectID       ProjectID  `json:"projectId,omitempty"`
	SelectedSkills  []SkillRef `json:"selectedSkills,omitempty"`
}

type UpdateInput struct {
	Title           *string     `json:"title,omitempty"`
	Cwd             *string     `json:"cwd,omitempty"`
	Provider        *Provider   `json:"provider,omitempty"`
	Model           *string     `json:"model,omitempty"`
	Mode            *string     `json:"mode,omitempty"`
	ReasoningEffort *string     `json:"reasoningEffort,omitempty"`
	ServiceTier     *string     `json:"serviceTier,omitempty"`
	SelectedSkills  *[]SkillRef `json:"selectedSkills,omitempty"`
}

func NormalizeProvider(provider Provider) Provider {
	switch provider {
	case ProviderClaude:
		return ProviderClaude
	case ProviderKimi:
		return ProviderKimi
	case ProviderAntigravity:
		return ProviderAntigravity
	default:
		return ProviderCodex
	}
}

func NormalizeReasoningEffort(effort string) string {
	return normalizeCapabilityValue(effort)
}

// NormalizeServiceTier keeps future provider tiers usable without requiring a
// frontend/backend release for every catalog addition. "" means Auto.
func NormalizeServiceTier(tier string) string {
	return normalizeCapabilityValue(tier)
}

func normalizeCapabilityValue(value string) string {
	value = strings.TrimSpace(value)
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' {
			continue
		}
		return ""
	}
	return value
}

func NormalizeSelectedSkills(skills []SkillRef, fallbackProvider Provider) []SkillRef {
	fallbackProvider = NormalizeProvider(fallbackProvider)
	seen := map[string]bool{}
	normalized := make([]SkillRef, 0, len(skills))
	for _, skill := range skills {
		name := strings.TrimSpace(skill.Name)
		command := strings.TrimSpace(skill.Command)
		source := strings.TrimSpace(skill.Source)
		if command == "" {
			command = name
		}
		if name == "" {
			name = command
		}
		if name == "" || command == "" {
			continue
		}

		provider := skill.Provider
		if provider == "" {
			provider = fallbackProvider
		} else {
			provider = NormalizeProvider(provider)
		}
		key := strings.ToLower(string(provider)) + "\x00" + strings.ToLower(source) + "\x00" + strings.ToLower(command)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, SkillRef{
			Name:     name,
			Command:  command,
			Provider: provider,
			Source:   source,
		})
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func ValidID(id ID) bool {
	if len(id) < 4 || len(id) > 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// TitleFromPrompt produces a short summary used when a chat is created with
// no explicit title. First 60 chars of the first prompt, single line.
func TitleFromPrompt(prompt string) string {
	t := strings.TrimSpace(prompt)
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ReplaceAll(t, "\r", " ")
	for strings.Contains(t, "  ") {
		t = strings.ReplaceAll(t, "  ", " ")
	}
	if len(t) > 60 {
		t = t[:60] + "..."
	}
	if t == "" {
		t = fmt.Sprintf("Chat %s", time.Now().Format("Jan 2 15:04"))
	}
	return t
}
