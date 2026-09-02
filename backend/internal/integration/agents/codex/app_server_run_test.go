package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestRunAppServerStreamsNativePlanTurn(t *testing.T) {
	script := `
while IFS= read -r line; do
  case "$line" in
    *'"id":1'*)
      printf '%s\n' '{"id":1,"result":{}}'
      ;;
    *'"id":2'*)
      printf '%s\n' '{"id":2,"result":{"thread":{"id":"thread-new"},"model":"gpt-test"}}'
      ;;
    *'"id":3'*)
      case "$line" in
        *'"mode":"plan"'*) ;;
        *) printf '%s\n' '{"id":3,"error":{"code":-1,"message":"missing native plan mode"}}'; exit 0 ;;
      esac
      printf '%s\n' '{"id":3,"result":{"turn":{"id":"turn-1","status":"inProgress","items":[]}}}'
      printf '%s\n' '{"method":"item/plan/delta","params":{"threadId":"thread-new","turnId":"turn-1","itemId":"plan-1","delta":"Native plan"}}'
      printf '%s\n' '{"method":"thread/tokenUsage/updated","params":{"tokenUsage":{"last":{"inputTokens":10,"cachedInputTokens":3,"cacheWriteInputTokens":0,"outputTokens":4,"reasoningOutputTokens":2}}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"thread-new","turn":{"id":"turn-1","status":"completed","items":[]}}}'
      exit 0
      ;;
  esac
done`

	var events []agent.Event
	err := runAppServer(
		context.Background(),
		exec.Command("sh", "-c", script),
		agent.RunRequest{ConversationID: "chat-1", Mode: agent.RunModePlan, Prompt: "plan it"},
		func(event agent.Event) { events = append(events, event) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Type != agent.EventSessionUpdated || events[0].SessionID != "thread-new" {
		t.Fatalf("session event = %#v", events[0])
	}
	if events[1].Type != agent.EventAssistantTextDelta || events[1].Text != "Native plan" {
		t.Fatalf("plan event = %#v", events[1])
	}
	if events[2].Type != agent.EventUsageUpdated {
		t.Fatalf("usage event = %#v", events[2])
	}
	if events[3].Type != agent.EventRunCompleted {
		t.Fatalf("completion event = %#v", events[3])
	}
	usage, ok := agent.ParseUsage(events[3].Usage)
	if !ok || usage.Model != "gpt-test" || usage.InputTokens != 7 ||
		usage.CacheReadTokens != 3 || usage.OutputTokens != 4 {
		t.Fatalf("completion usage = %#v (ok=%t)", usage, ok)
	}
}

func TestRunAppServerMapsMissingResumeThread(t *testing.T) {
	script := `
while IFS= read -r line; do
  case "$line" in
    *'"id":1'*) printf '%s\n' '{"id":1,"result":{}}' ;;
    *'"id":2'*) printf '%s\n' '{"id":2,"error":{"code":-1,"message":"thread not found"}}'; exit 0 ;;
  esac
done`

	err := runAppServer(
		context.Background(),
		exec.Command("sh", "-c", script),
		agent.RunRequest{ResumeID: "missing", Prompt: "continue"},
		func(agent.Event) {},
	)
	if !errors.Is(err, agent.ErrSessionNotFound) {
		t.Fatalf("error = %v, want ErrSessionNotFound", err)
	}
}

func TestAppServerRequestStaysPendingUntilExplicitResponse(t *testing.T) {
	var encoded strings.Builder
	var events []agent.Event
	handler := newAppServerRequestHandler(
		agent.RunRequest{Mode: agent.RunModePlan},
		func(event agent.Event) { events = append(events, event) },
		func(message any) error {
			data, marshalErr := json.Marshal(message)
			if marshalErr == nil {
				encoded.Write(data)
			}
			return marshalErr
		},
	)
	err := handler.Handle(appServerEnvelope{
		ID:     []byte(`"request-42"`),
		Method: "item/fileChange/requestApproval",
		Params: []byte(`{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if encoded.Len() != 0 {
		t.Fatalf("request was answered before user input: %s", encoded.String())
	}
	if len(events) != 1 || events[0].Type != agent.EventInteractionRequest || events[0].InteractionID != `"request-42"` {
		t.Fatalf("events = %#v", events)
	}
	if err := handler.Respond(agent.InteractionResponse{
		ID: `"request-42"`, Result: []byte(`{"decision":"decline"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), `"id":"request-42"`) || !strings.Contains(encoded.String(), `"decision":"decline"`) {
		t.Fatalf("response = %s", encoded.String())
	}
	if len(events) != 2 || events[1].Type != agent.EventInteractionDone || events[1].Status != "denied" {
		t.Fatalf("events = %#v", events)
	}
}

func TestAppServerDoesNotPersistSecretInteractionAnswers(t *testing.T) {
	var events []agent.Event
	handler := newAppServerRequestHandler(
		agent.RunRequest{ConversationID: "chat-1"},
		func(event agent.Event) { events = append(events, event) },
		func(any) error { return nil },
	)
	if err := handler.Handle(appServerEnvelope{
		ID:     []byte("9"),
		Method: "item/tool/requestUserInput",
		Params: []byte(`{"questions":[{"id":"token","question":"Token?","isSecret":true,"options":[]}]}`),
	}); err != nil {
		t.Fatal(err)
	}
	if err := handler.Respond(agent.InteractionResponse{
		ID: "9", Result: []byte(`{"answers":{"token":{"answers":["super-secret-value"]}}}`),
	}); err != nil {
		t.Fatal(err)
	}
	persisted, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "super-secret-value") {
		t.Fatalf("secret leaked into events: %s", persisted)
	}
}

func TestRunAppServerUsesNativeTurnInterrupt(t *testing.T) {
	script := `
while IFS= read -r line; do
  case "$line" in
    *'"id":1'*) printf '%s\n' '{"id":1,"result":{}}' ;;
    *'"id":2'*) printf '%s\n' '{"id":2,"result":{"thread":{"id":"thread-1"},"model":"gpt-test"}}' ;;
    *'"id":3'*)
      printf '%s\n' '{"id":3,"result":{"turn":{"id":"turn-1","status":"inProgress","items":[]}}}'
      printf '%s\n' '{"method":"turn/started","params":{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress","items":[]}}}'
      ;;
    *'"id":4'*)
      case "$line" in
        *'"method":"turn/interrupt"'*'"threadId":"thread-1"'*'"turnId":"turn-1"'*) ;;
        *) exit 7 ;;
      esac
      printf '%s\n' '{"id":4,"result":{}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-1","status":"interrupted","items":[]}}}'
      exit 0
      ;;
  esac
done`

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []agent.Event
	err := runAppServer(
		ctx,
		exec.Command("sh", "-c", script),
		agent.RunRequest{ConversationID: "chat-1", Prompt: "work"},
		func(event agent.Event) {
			events = append(events, event)
			if event.Type == agent.EventTurnStatus && event.Status == "inProgress" {
				cancel()
			}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[len(events)-1].Type != agent.EventRunInterrupted {
		t.Fatalf("events = %#v", events)
	}
}
