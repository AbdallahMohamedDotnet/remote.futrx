package prompt

import (
	"encoding/json"
	"testing"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent"
	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
)

func TestChatEventFromAgentEventMapsSession(t *testing.T) {
	ev, ok := chatEventFromAgentEvent(agent.Event{
		T:         123,
		Type:      agent.EventSessionUpdated,
		SessionID: "claude-session",
	})
	if !ok {
		t.Fatal("expected event to map")
	}
	if ev.Type != "session" || ev.ClaudeSessionID != "claude-session" || ev.T != 123 {
		t.Fatalf("unexpected chat event: %#v", ev)
	}
}

func TestChatEventFromAgentEventMapsCodexSession(t *testing.T) {
	ev, ok := chatEventFromAgentEvent(agent.Event{
		T:         123,
		Type:      agent.EventSessionUpdated,
		Provider:  agent.ProviderCodex,
		SessionID: "codex-thread",
	})
	if !ok {
		t.Fatal("expected event to map")
	}
	if ev.Type != "session" || ev.CodexSessionID != "codex-thread" || ev.Provider != servicechat.ProviderCodex {
		t.Fatalf("unexpected chat event: %#v", ev)
	}
}

func TestChatEventFromAgentEventMapsToolLifecycle(t *testing.T) {
	input := json.RawMessage(`{"cmd":"go test ./..."}`)
	start, ok := chatEventFromAgentEvent(agent.Event{
		T:        456,
		Type:     agent.EventToolStarted,
		ItemID:   "tool-1",
		ToolName: "Bash",
		Input:    input,
	})
	if !ok {
		t.Fatal("expected start event to map")
	}
	if start.Type != "tool_use_start" || start.ID != "tool-1" || start.Name != "Bash" || string(start.Input) != string(input) {
		t.Fatalf("unexpected start event: %#v", start)
	}

	end, ok := chatEventFromAgentEvent(agent.Event{
		T:      789,
		Type:   agent.EventToolCompleted,
		ItemID: "tool-1",
		Output: "ok",
	})
	if !ok {
		t.Fatal("expected end event to map")
	}
	if end.Type != "tool_use_end" || end.ID != "tool-1" || end.Output != "ok" || end.IsError {
		t.Fatalf("unexpected end event: %#v", end)
	}
}
