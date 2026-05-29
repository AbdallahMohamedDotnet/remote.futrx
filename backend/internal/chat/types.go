package chat

import "encoding/json"

// ChatMeta lives at data/chats/{id}/meta.json and is updated whenever the
// claude session id or last-message timestamp changes.
type ChatMeta struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	ClaudeSessionID string `json:"claudeSessionId,omitempty"`
	TmuxSession     string `json:"tmuxSession,omitempty"`
	Cwd             string `json:"cwd,omitempty"`
	CreatedAt       int64  `json:"createdAt"`
	LastMessageAt   int64  `json:"lastMessageAt"`
	Model           string `json:"model,omitempty"`
	Mode            string `json:"mode,omitempty"`
	// ProjectID links a chat to a project. Empty = legacy chat (runs claude on
	// the host); non-empty = future container-spawn target. Wired in task #9.
	ProjectID string `json:"projectId,omitempty"`
}

// ChatEvent is the normalized shape we persist and stream. It mirrors the
// TypeScript ChatEvent union in src/types.ts.
type ChatEvent struct {
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
	Usage           json.RawMessage `json:"usage,omitempty"`
	Message         string          `json:"message,omitempty"`
	Running         bool            `json:"running,omitempty"`
}
