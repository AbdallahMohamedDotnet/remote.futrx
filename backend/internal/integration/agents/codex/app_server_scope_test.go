package codex

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestRunAppServerScopesNotificationsToRequestedTurn(t *testing.T) {
	for _, test := range []struct {
		name           string
		resumeID       string
		fork           bool
		method         string
		beforeResponse bool
	}{
		{name: "new", method: "thread/start"},
		{name: "resumed", resumeID: "parent", method: "thread/resume"},
		{name: "forked", resumeID: "source", fork: true, method: "thread/fork"},
		{name: "notifications before response", method: "thread/start", beforeResponse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A second subagent requests approval after the first one completes.
			// The parent can only finish if Remote keeps stdin open and continues
			// answering requests, even though child notifications are not displayed.
			script := `
while IFS= read -r line; do
  case "$line" in
    *'"id":1'*) printf '%s\n' '{"id":1,"result":{}}' ;;
    *'"id":2'*)
      case "$line" in
        *'"method":"__THREAD_METHOD__"'*) ;;
        *) exit 1 ;;
      esac
      printf '%s\n' '{"id":2,"result":{"thread":{"id":"parent"},"model":"gpt-test"}}'
      ;;
    *'"id":3'*)
      __EARLY_RESPONSE__
      printf '%s\n' '{"method":"turn/started","params":{"threadId":"parent","turn":{"id":"parent-turn","status":"inProgress"}}}'
      printf '%s\n' '{"method":"turn/started","params":{"threadId":"child","turn":{"id":"child-turn","status":"inProgress"}}}'
      printf '%s\n' '{"method":"item/agentMessage/delta","params":{"threadId":"parent","turnId":"parent-turn","itemId":"message","delta":"Parent "}}'
      printf '%s\n' '{"method":"item/agentMessage/delta","params":{"threadId":"child","turnId":"child-turn","itemId":"message","delta":"Research complete and sent to the parent agent."}}'
      printf '%s\n' '{"method":"item/agentMessage/delta","params":{"threadId":"child","turnId":"parent-turn","itemId":"message","delta":"Wrong thread"}}'
      printf '%s\n' '{"method":"item/completed","params":{"threadId":"child","turnId":"child-turn","item":{"id":"message","type":"agentMessage","text":"Research complete and sent to the parent agent."}}}'
      printf '%s\n' '{"method":"item/plan/delta","params":{"threadId":"parent","turnId":"old-turn","itemId":"plan","delta":"Old plan"}}'
      printf '%s\n' '{"method":"item/reasoning/textDelta","params":{"threadId":"child","turnId":"child-turn","itemId":"reasoning","delta":"Child reasoning"}}'
      printf '%s\n' '{"method":"item/started","params":{"threadId":"child","turnId":"child-turn","item":{"id":"command","type":"commandExecution","command":"pwd"}}}'
      printf '%s\n' '{"method":"item/completed","params":{"threadId":"child","turnId":"child-turn","item":{"id":"command","type":"commandExecution","aggregatedOutput":"/workspace","exitCode":0}}}'
      printf '%s\n' '{"method":"error","params":{"threadId":"child","turnId":"child-turn","message":"Child error"}}'
      printf '%s\n' '{"method":"thread/tokenUsage/updated","params":{"threadId":"parent","turnId":"parent-turn","tokenUsage":{"last":{"inputTokens":10,"cachedInputTokens":3,"outputTokens":4}}}}'
      printf '%s\n' '{"method":"thread/tokenUsage/updated","params":{"threadId":"child","turnId":"child-turn","tokenUsage":{"last":{"inputTokens":999,"outputTokens":999}}}}'
      printf '%s\n' '{"method":"thread/tokenUsage/updated","params":{"threadId":"parent","turnId":"old-turn","tokenUsage":{"last":{"inputTokens":888,"outputTokens":888}}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"child","turn":{"id":"child-turn","status":"completed"}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"child","turn":{"id":"child-turn","status":"failed","error":{"message":"Child failed"}}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"child","turn":{"id":"child-turn","status":"interrupted"}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"parent","turn":{"id":"old-turn","status":"completed"}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"turn":{"id":"parent-turn","status":"completed"}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"parent","turn":{"status":"completed"}}}'
      __LATE_RESPONSE__
      printf '%s\n' '{"id":99,"method":"item/commandExecution/requestApproval","params":{"threadId":"other-child","turnId":"other-child-turn","itemId":"approval","command":"pwd"}}'
      ;;
    *'"id":99'*)
      case "$line" in
        *'"decision":"accept"'*) ;;
        *) exit 1 ;;
      esac
      printf '%s\n' '{"method":"item/completed","params":{"threadId":"parent","turnId":"parent-turn","item":{"id":"message","type":"agentMessage","text":"Parent final answer"}}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"parent","turn":{"id":"parent-turn","status":"completed"}}}'
      exit 0
      ;;
  esac
done`
			response := `printf '%s\n' '{"id":3,"result":{"turn":{"id":"parent-turn","status":"inProgress","items":[]}}}'`
			early, late := response, ""
			if test.beforeResponse {
				early, late = "", response
			}
			script = strings.NewReplacer(
				"__THREAD_METHOD__", test.method,
				"__EARLY_RESPONSE__", early,
				"__LATE_RESPONSE__", late,
			).Replace(script)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var events []agent.Event
			err := runAppServer(ctx, exec.CommandContext(ctx, "sh", "-c", script), agent.RunRequest{
				ConversationID: "chat-1", Prompt: "research", ResumeID: test.resumeID, Fork: test.fork,
			}, func(event agent.Event) { events = append(events, event) })
			if err != nil {
				t.Fatal(err)
			}
			if ctx.Err() != nil {
				t.Fatal("app-server did not answer the subagent request and finish the parent turn")
			}
			if test.resumeID != "parent" {
				if len(events) == 0 || events[0].Type != agent.EventSessionUpdated || events[0].SessionID != "parent" {
					t.Fatalf("missing parent session: %#v", events)
				}
				events = events[1:]
			}
			if len(events) != 3 ||
				events[0].Type != agent.EventAssistantTextDelta || events[0].Text != "Parent " ||
				events[1].Type != agent.EventAssistantTextDelta || events[1].Text != "final answer" ||
				events[2].Type != agent.EventRunCompleted {
				for _, event := range events {
					t.Logf("%s: text=%q message=%q usage=%s", event.Type, event.Text, event.Message, event.Usage)
				}
				t.Fatal("expected only the parent answer and completion")
			}
			usage, ok := agent.ParseUsage(events[2].Usage)
			if !ok || usage.InputTokens != 7 || usage.CacheReadTokens != 3 || usage.OutputTokens != 4 {
				t.Fatalf("parent usage was replaced: %#v (ok=%t)", usage, ok)
			}
		})
	}
}

func TestRunAppServerRequiresParentCompletion(t *testing.T) {
	for _, test := range []struct {
		name          string
		notifications string
		wantEvent     agent.EventType
		wantError     string
	}{
		{
			name: "child completion is not success on EOF",
			notifications: `{"id":3,"result":{"turn":{"id":"parent-turn","status":"inProgress"}}}
{"method":"turn/completed","params":{"threadId":"child","turn":{"id":"child-turn","status":"completed"}}}`,
			wantError: "closed before the turn completed",
		},
		{
			name: "parent failure after child success",
			notifications: `{"id":3,"result":{"turn":{"id":"parent-turn","status":"inProgress"}}}
{"method":"turn/completed","params":{"threadId":"child","turn":{"id":"child-turn","status":"completed"}}}
{"method":"turn/completed","params":{"threadId":"parent","turn":{"id":"parent-turn","status":"failed","error":{"message":"Parent failed"}}}}`,
			wantEvent: agent.EventRunFailed,
			wantError: agent.ErrRunFailed.Error(),
		},
		{
			name: "parent completion before response",
			notifications: `{"method":"turn/completed","params":{"threadId":"parent","turn":{"id":"parent-turn","status":"completed"}}}
{"method":"item/agentMessage/delta","params":{"threadId":"parent","turnId":"parent-turn","itemId":"late","delta":"After completion"}}
{"id":3,"result":{"turn":{"id":"parent-turn","status":"inProgress"}}}`,
			wantEvent: agent.EventRunCompleted,
		},
		{
			name:          "missing turn ID fails safely",
			notifications: `{"id":3,"result":{"turn":{"status":"inProgress"}}}`,
			wantError:     "turn without an id",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			script := `
while IFS= read -r line; do
  case "$line" in
    *'"id":1'*) printf '%s\n' '{"id":1,"result":{}}' ;;
    *'"id":2'*) printf '%s\n' '{"id":2,"result":{"thread":{"id":"parent"},"model":"gpt-test"}}' ;;
    *'"id":3'*)
      cat <<'NOTIFICATIONS'
__NOTIFICATIONS__
NOTIFICATIONS
      exit 0
      ;;
  esac
done`
			script = strings.ReplaceAll(script, "__NOTIFICATIONS__", test.notifications)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var events []agent.Event
			err := runAppServer(ctx, exec.CommandContext(ctx, "sh", "-c", script), agent.RunRequest{
				ConversationID: "chat-1", ResumeID: "parent", Prompt: "research",
			}, func(event agent.Event) { events = append(events, event) })
			if test.wantError == "" && err != nil || test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
			if test.wantEvent == "" {
				if len(events) != 0 {
					t.Fatalf("unexpected events: %d", len(events))
				}
			} else if len(events) != 1 || events[0].Type != test.wantEvent {
				t.Fatalf("expected one %s event, got %d events", test.wantEvent, len(events))
			}
			if test.wantEvent == agent.EventRunFailed && (!errors.Is(err, agent.ErrRunFailed) || events[0].Message != "Parent failed") {
				t.Fatalf("parent failure was lost: %v", err)
			}
		})
	}
}
