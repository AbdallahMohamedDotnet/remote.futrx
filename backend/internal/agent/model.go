package agent

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrRunFailed = errors.New("agent run failed")
var ErrSessionNotFound = errors.New("agent session not found")

type ProviderID string

const (
	ProviderClaude      ProviderID = "claude"
	ProviderCodex       ProviderID = "codex"
	ProviderKimi        ProviderID = "kimi"
	ProviderAntigravity ProviderID = "antigravity"
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

type ReasoningEffort string
type ServiceTier string

type CapabilitySource string

const (
	CapabilitySourceLive     CapabilitySource = "live"
	CapabilitySourceFallback CapabilitySource = "fallback"
)

// CapabilityOption is the provider-neutral shape used by model, reasoning,
// speed, and mode selectors. Native distinguishes a provider-owned mode from a
// Remote workflow preset implemented through prompt instructions.
type CapabilityOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Native      bool   `json:"native,omitempty"`
}

// ModelCapability keeps controls next to the model that supports them. Some
// providers only publish provider-wide controls; their adapter expands those
// controls onto each returned model before exposing the catalog.
type ModelCapability struct {
	ID                     string             `json:"id"`
	Label                  string             `json:"label"`
	Description            string             `json:"description,omitempty"`
	ProviderDefault        bool               `json:"providerDefault,omitempty"`
	ReasoningEfforts       []CapabilityOption `json:"reasoningEfforts"`
	DefaultReasoningEffort string             `json:"defaultReasoningEffort,omitempty"`
	ServiceTiers           []CapabilityOption `json:"serviceTiers"`
	DefaultServiceTier     string             `json:"defaultServiceTier,omitempty"`
}

// Capabilities is the normalized catalog returned by every agent adapter.
// Warning is intentionally concise and must not contain raw provider output.
type Capabilities struct {
	Provider    ProviderID         `json:"provider"`
	Label       string             `json:"label"`
	Version     string             `json:"version,omitempty"`
	Source      CapabilitySource   `json:"source"`
	Warning     string             `json:"warning,omitempty"`
	Models      []ModelCapability  `json:"models"`
	Modes       []CapabilityOption `json:"modes"`
	DefaultMode string             `json:"defaultMode,omitempty"`
}

type CapabilityRequest struct {
	// ContainerName scopes discovery to the project computer and therefore to
	// its installed CLI, credentials, and account entitlements. Empty means the
	// provider should inspect the host CLI.
	ContainerName string
}

// RunPreferences contains provider-neutral launch preferences. Provider
// adapters remain responsible for accepting only the values their CLI supports.
type RunPreferences struct {
	ReasoningEffort ReasoningEffort
	ServiceTier     ServiceTier
}

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
	Preferences    RunPreferences
	// EnableBrowser wires the Agent Browser MCP tools into the agent launch.
	// Set when the `browser` skill is selected for the prompt.
	EnableBrowser bool
	// EnableScheduleTools ensures the provider-neutral remote-schedule CLI and
	// its skill are present for this run.
	EnableScheduleTools bool
	// RuntimeEnv carries short-lived, backend-issued capabilities into a run.
	// Provider adapters must not persist these values in project configuration.
	RuntimeEnv map[string]string
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
	Capabilities(ctx context.Context, req CapabilityRequest) (Capabilities, error)
}
