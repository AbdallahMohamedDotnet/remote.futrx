package filechat

import (
	"encoding/json"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
)

type metaRecord struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	ClaudeSessionID string `json:"claudeSessionId,omitempty"`
	TmuxSession     string `json:"tmuxSession,omitempty"`
	Cwd             string `json:"cwd,omitempty"`
	CreatedAt       int64  `json:"createdAt"`
	LastMessageAt   int64  `json:"lastMessageAt"`
	Model           string `json:"model,omitempty"`
	Mode            string `json:"mode,omitempty"`
	ProjectID       string `json:"projectId,omitempty"`
}

func metaRecordFromDomain(m servicechat.Meta) metaRecord {
	return metaRecord{
		ID:              string(m.ID),
		Title:           m.Title,
		ClaudeSessionID: m.ClaudeSessionID,
		TmuxSession:     m.TmuxSession,
		Cwd:             m.Cwd,
		CreatedAt:       m.CreatedAt,
		LastMessageAt:   m.LastMessageAt,
		Model:           m.Model,
		Mode:            m.Mode,
		ProjectID:       string(m.ProjectID),
	}
}

func (r metaRecord) toDomain() servicechat.Meta {
	return servicechat.Meta{
		ID:              servicechat.ID(r.ID),
		Title:           r.Title,
		ClaudeSessionID: r.ClaudeSessionID,
		TmuxSession:     r.TmuxSession,
		Cwd:             r.Cwd,
		CreatedAt:       r.CreatedAt,
		LastMessageAt:   r.LastMessageAt,
		Model:           r.Model,
		Mode:            r.Mode,
		ProjectID:       servicechat.ProjectID(r.ProjectID),
	}
}

type eventRecord struct {
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

func eventRecordFromDomain(ev servicechat.Event) eventRecord {
	return eventRecord{
		T:               ev.T,
		Type:            ev.Type,
		Text:            ev.Text,
		MessageID:       ev.MessageID,
		ID:              ev.ID,
		Name:            ev.Name,
		Input:           ev.Input,
		Output:          ev.Output,
		IsError:         ev.IsError,
		ToolName:        ev.ToolName,
		Subtype:         ev.Subtype,
		Data:            ev.Data,
		ClaudeSessionID: ev.ClaudeSessionID,
		Usage:           ev.Usage,
		Message:         ev.Message,
		Running:         ev.Running,
	}
}

func (r eventRecord) toDomain() servicechat.Event {
	return servicechat.Event{
		T:               r.T,
		Type:            r.Type,
		Text:            r.Text,
		MessageID:       r.MessageID,
		ID:              r.ID,
		Name:            r.Name,
		Input:           r.Input,
		Output:          r.Output,
		IsError:         r.IsError,
		ToolName:        r.ToolName,
		Subtype:         r.Subtype,
		Data:            r.Data,
		ClaudeSessionID: r.ClaudeSessionID,
		Usage:           r.Usage,
		Message:         r.Message,
		Running:         r.Running,
	}
}
