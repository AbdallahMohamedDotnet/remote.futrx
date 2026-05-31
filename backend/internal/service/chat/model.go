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
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
)

type Meta struct {
	ID              ID        `json:"id"`
	Title           string    `json:"title"`
	Provider        Provider  `json:"provider,omitempty"`
	ClaudeSessionID string    `json:"claudeSessionId,omitempty"`
	CodexSessionID  string    `json:"codexSessionId,omitempty"`
	TmuxSession     string    `json:"tmuxSession,omitempty"`
	Cwd             string    `json:"cwd,omitempty"`
	CreatedAt       int64     `json:"createdAt"`
	LastMessageAt   int64     `json:"lastMessageAt"`
	LastReadAt      int64     `json:"lastReadAt,omitempty"`
	Running         bool      `json:"running,omitempty"`
	Model           string    `json:"model,omitempty"`
	Mode            string    `json:"mode,omitempty"`
	ReasoningEffort string    `json:"reasoningEffort,omitempty"`
	ProjectID       ProjectID `json:"projectId,omitempty"`
}

type Event struct {
	Seq             int64           `json:"seq,omitempty"`
	T               int64           `json:"t"`
	Type            string          `json:"type"`
	Text            string          `json:"text,omitempty"`
	MessageID       string          `json:"messageId,omitempty"`
	ID              string          `json:"id,omitempty"`
	Name            string          `json:"name,omitempty"`
	Input           json.RawMessage `json:"input,omitempty"`
	Output          string          `json:"output,omitempty"`
	IsError         bool            `json:"isError,omitempty"`
	ToolName        string          `json:"toolName,omitempty"`
	Subtype         string          `json:"subtype,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
	ClaudeSessionID string          `json:"claudeSessionId,omitempty"`
	CodexSessionID  string          `json:"codexSessionId,omitempty"`
	Provider        Provider        `json:"provider,omitempty"`
	Usage           json.RawMessage `json:"usage,omitempty"`
	Message         string          `json:"message,omitempty"`
	Running         bool            `json:"running,omitempty"`
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
	Title           string    `json:"title,omitempty"`
	TmuxSession     string    `json:"tmuxSession,omitempty"`
	Cwd             string    `json:"cwd,omitempty"`
	Provider        Provider  `json:"provider,omitempty"`
	Model           string    `json:"model,omitempty"`
	Mode            string    `json:"mode,omitempty"`
	ReasoningEffort string    `json:"reasoningEffort,omitempty"`
	ProjectID       ProjectID `json:"projectId,omitempty"`
}

type UpdateInput struct {
	Title           *string   `json:"title,omitempty"`
	Cwd             *string   `json:"cwd,omitempty"`
	Provider        *Provider `json:"provider,omitempty"`
	Model           *string   `json:"model,omitempty"`
	Mode            *string   `json:"mode,omitempty"`
	ReasoningEffort *string   `json:"reasoningEffort,omitempty"`
}

func NormalizeProvider(provider Provider) Provider {
	switch provider {
	case ProviderCodex:
		return ProviderCodex
	default:
		return ProviderClaude
	}
}

func NormalizeReasoningEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	case "xhigh":
		return "xhigh"
	default:
		return ""
	}
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
