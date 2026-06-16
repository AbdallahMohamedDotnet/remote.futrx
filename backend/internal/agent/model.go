package agent

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrRunFailed = errors.New("agent run failed")

type ProviderID string

const (
	ProviderClaude ProviderID = "claude"
	ProviderCodex  ProviderID = "codex"
)

type EventType string

const (
	EventRunStarted         EventType = "run.started"
	EventRunCompleted       EventType = "run.completed"
	EventRunFailed          EventType = "run.failed"
	EventSessionUpdated     EventType = "session.updated"
	EventSystem             EventType = "system"
	EventAssistantTextDelta EventType = "assistant.delta"
	EventReasoningDelta     EventType = "reasoning.delta"
	EventToolStarted        EventType = "tool.started"
	EventToolCompleted      EventType = "tool.completed"
	EventUsageUpdated       EventType = "usage.updated"
	EventError              EventType = "error"
)

type ItemKind string

const (
	ItemMessage   ItemKind = "message"
	ItemReasoning ItemKind = "reasoning"
	ItemToolCall  ItemKind = "tool_call"
	ItemSystem    ItemKind = "system"
)

// RunRequest is provider-neutral. Provider adapters translate it into the
// concrete CLI flags and runtime setup required by Claude Code, Codex, etc.
type RunRequest struct {
	Provider       ProviderID
	ConversationID string
	Prompt         string
	Cwd            string
	Model          string
	Mode           string
	ResumeID       string
	ProjectID      string
	Fork           bool
	Config         map[string]any
	// EnableBrowser wires the @playwright/mcp browser tools into the agent
	// launch. Set when the `browser` skill is selected for the prompt.
	EnableBrowser bool
}

// Event is the normalized backend event shape emitted by headless agent
// providers. Transport-specific chat events are derived from this at the edge.
type Event struct {
	T              int64           `json:"t"`
	Type           EventType       `json:"type"`
	Provider       ProviderID      `json:"provider,omitempty"`
	ConversationID string          `json:"conversationId,omitempty"`
	RunID          string          `json:"runId,omitempty"`
	SessionID      string          `json:"sessionId,omitempty"`
	MessageID      string          `json:"messageId,omitempty"`
	ItemID         string          `json:"itemId,omitempty"`
	ItemKind       ItemKind        `json:"itemKind,omitempty"`
	Role           string          `json:"role,omitempty"`
	Text           string          `json:"text,omitempty"`
	Message        string          `json:"message,omitempty"`
	Subtype        string          `json:"subtype,omitempty"`
	Model          string          `json:"model,omitempty"`
	ToolName       string          `json:"toolName,omitempty"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         string          `json:"output,omitempty"`
	IsError        bool            `json:"isError,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
	Usage          json.RawMessage `json:"usage,omitempty"`
	Raw            json.RawMessage `json:"raw,omitempty"`
}

type Provider interface {
	ID() ProviderID
	Parser(req RunRequest) LineParser
	Run(ctx context.Context, req RunRequest, emit func(Event)) error
}
