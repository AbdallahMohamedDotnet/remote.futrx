package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

type testPendingInteractionFunc func() (agent.InteractionResponse, error)

func (await testPendingInteractionFunc) Await() (agent.InteractionResponse, error) {
	return await()
}

type testInteractionHandlerFunc func(
	context.Context,
	agent.InteractionRequest,
) (agent.PendingInteraction, error)

func (begin testInteractionHandlerFunc) BeginInteraction(
	ctx context.Context,
	request agent.InteractionRequest,
) (agent.PendingInteraction, error) {
	return begin(ctx, request)
}

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
      printf '%s\n' '{"method":"thread/tokenUsage/updated","params":{"threadId":"thread-new","turnId":"turn-1","tokenUsage":{"last":{"inputTokens":10,"cachedInputTokens":3,"cacheWriteInputTokens":0,"outputTokens":4,"reasoningOutputTokens":2}}}}'
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
	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Type != agent.EventSessionUpdated || events[0].SessionID != "thread-new" {
		t.Fatalf("session event = %#v", events[0])
	}
	if events[1].Type != agent.EventAssistantTextDelta || events[1].Text != "Native plan" {
		t.Fatalf("plan event = %#v", events[1])
	}
	if events[2].Type != agent.EventRunCompleted {
		t.Fatalf("completion event = %#v", events[2])
	}
	usage, ok := agent.ParseUsage(events[2].Usage)
	if !ok || usage.Model != "gpt-test" || usage.InputTokens != 7 ||
		usage.CacheReadTokens != 3 || usage.OutputTokens != 4 {
		t.Fatalf("completion usage = %#v (ok=%t)", usage, ok)
	}
}

func TestRunAppServerKeepsStreamingDuringCorrelatedUserInput(t *testing.T) {
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
      printf '%s\n' '{"id":3,"result":{"turn":{"id":"turn-1","status":"inProgress","items":[]}}}'
      printf '%s\n' '{"id":99,"method":"item/tool/requestUserInput","params":{"threadId":"thread-new","turnId":"turn-1","itemId":"question-item","isBlocking":true,"autoResolutionMs":null,"questions":[{"id":"choice","header":"Choice","question":"Choose one","isOther":false,"isSecret":false,"options":[{"label":"A","description":"first"}]}]}}'
      printf '%s\n' '{"method":"item/agentMessage/delta","params":{"threadId":"thread-new","turnId":"turn-1","itemId":"message-1","delta":"still streaming"}}'
      ;;
    *'"id":99'*)
      case "$line" in
        *'"choice":{"answers":["A"]}'*) ;;
        *) printf '%s\n' '{"method":"error","params":{"message":"missing correlated answer"}}'; exit 0 ;;
      esac
      printf '%s\n' '{"method":"serverRequest/resolved","params":{"threadId":"thread-new","requestId":99}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"thread-new","turn":{"id":"turn-1","status":"completed","items":[]}}}'
      exit 0
      ;;
  esac
done`

	interaction := make(chan agent.InteractionRequest, 1)
	allowAnswer := make(chan struct{})
	events := make(chan agent.Event, 8)
	done := make(chan error, 1)
	go func() {
		done <- runAppServer(
			context.Background(),
			exec.Command("sh", "-c", script),
			agent.RunRequest{
				ConversationID: "chat-1",
				Prompt:         "ask me",
				Interactions: testInteractionHandlerFunc(func(
					_ context.Context,
					request agent.InteractionRequest,
				) (agent.PendingInteraction, error) {
					interaction <- request
					return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
						<-allowAnswer
						return agent.InteractionResponse{
							Answers: map[string][]string{"choice": {"A"}},
						}, nil
					}), nil
				}),
			},
			func(event agent.Event) { events <- event },
		)
	}()

	select {
	case request := <-interaction:
		if request.ID != "question-item" || !request.Blocking {
			t.Fatalf("interaction = %#v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("app-server did not surface user input")
	}

	foundStreamingDelta := false
	deadline := time.After(time.Second)
	for !foundStreamingDelta {
		select {
		case event := <-events:
			foundStreamingDelta = event.Type == agent.EventAssistantTextDelta && event.Text == "still streaming"
		case <-deadline:
			t.Fatal("stdout scanner stopped while user input was pending")
		}
	}
	close(allowAnswer)

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("app-server did not finish after interaction response")
	}
}

func TestRunAppServerCancelsResolvedRequestWithoutLateResponse(t *testing.T) {
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
      printf '%s\n' '{"id":3,"result":{"turn":{"id":"turn-1","status":"inProgress","items":[]}}}'
      printf '%s\n' '{"id":99,"method":"item/tool/requestUserInput","params":{"threadId":"child","turnId":"child-turn","itemId":"question-item","isBlocking":false,"questions":[{"id":"choice","question":"Choose","isOther":false,"options":null}]}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"other-child","turn":{"id":"other-child-turn","status":"completed"}}}'
      printf '%s\n' '{"method":"item/agentMessage/delta","params":{"threadId":"thread-new","turnId":"turn-1","itemId":"message","delta":"Parent still running"}}'
      printf '%s\n' '{"method":"serverRequest/resolved","params":{"threadId":"child","requestId":99}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"thread-new","turn":{"id":"turn-1","status":"completed","items":[]}}}'
      exit 0
      ;;
    *'"id":99'*)
      printf '%s\n' '{"method":"error","params":{"message":"late response after native resolution"}}'
      exit 1
      ;;
  esac
done`

	registered := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)
	order := make(chan string, 4)
	var events []agent.Event
	err := runAppServer(
		context.Background(),
		exec.Command("sh", "-c", script),
		agent.RunRequest{
			ConversationID: "chat-1",
			Prompt:         "ask me",
			Interactions: testInteractionHandlerFunc(func(
				ctx context.Context,
				request agent.InteractionRequest,
			) (agent.PendingInteraction, error) {
				order <- "request"
				registered <- struct{}{}
				return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
					<-ctx.Done()
					order <- "resolved"
					cancelled <- struct{}{}
					return agent.InteractionResponse{}, ctx.Err()
				}), nil
			}),
		},
		func(event agent.Event) {
			events = append(events, event)
			if event.Type == agent.EventAssistantTextDelta {
				order <- "streaming"
			}
			if event.Type == agent.EventRunCompleted {
				order <- "complete"
			}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-registered:
	default:
		t.Fatal("request was never registered")
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("native resolution did not cancel interaction handler")
	}
	if len(events) == 0 || events[len(events)-1].Type != agent.EventRunCompleted {
		t.Fatalf("events = %#v", events)
	}
	for index, want := range []string{"request", "streaming", "resolved", "complete"} {
		select {
		case got := <-order:
			if got != want {
				t.Fatalf("order[%d] = %q, want %q", index, got, want)
			}
		default:
			t.Fatalf("order stopped before %q", want)
		}
	}
}

func TestRunAppServerCancelsPendingRequestBeforeTerminalWithoutResolvedNotice(t *testing.T) {
	script := `
while IFS= read -r line; do
  case "$line" in
    *'"id":1'*) printf '%s\n' '{"id":1,"result":{}}' ;;
    *'"id":2'*) printf '%s\n' '{"id":2,"result":{"thread":{"id":"thread-new"},"model":"gpt-test"}}' ;;
    *'"id":3'*)
      printf '%s\n' '{"id":3,"result":{"turn":{"id":"turn-1","status":"inProgress","items":[]}}}'
      printf '%s\n' '{"id":99,"method":"item/tool/requestUserInput","params":{"itemId":"question-item","isBlocking":true,"questions":[{"id":"choice","question":"Choose","options":[]}]}}'
      printf '%s\n' '{"method":"turn/completed","params":{"threadId":"thread-new","turn":{"id":"turn-1","status":"completed","items":[]}}}'
      exit 0
      ;;
  esac
done`

	order := make(chan string, 3)
	err := runAppServer(
		context.Background(),
		exec.Command("sh", "-c", script),
		agent.RunRequest{
			ConversationID: "chat-1",
			Prompt:         "ask me",
			Interactions: testInteractionHandlerFunc(func(ctx context.Context, _ agent.InteractionRequest) (agent.PendingInteraction, error) {
				order <- "request"
				return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
					<-ctx.Done()
					order <- "resolved"
					return agent.InteractionResponse{}, ctx.Err()
				}), nil
			}),
		},
		func(event agent.Event) {
			if event.Type == agent.EventRunCompleted {
				order <- "complete"
			}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []string{"request", "resolved", "complete"} {
		select {
		case got := <-order:
			if got != want {
				t.Fatalf("order[%d] = %q, want %q", index, got, want)
			}
		default:
			t.Fatalf("order stopped before %q", want)
		}
	}
}

func TestAppServerRequestKeyPreservesJSONRPCIDType(t *testing.T) {
	if numeric, text := appServerRequestKey(json.RawMessage(`1`)), appServerRequestKey(json.RawMessage(`"1"`)); numeric == text || numeric != "n:1" || text != "s:1" {
		t.Fatalf("numeric = %q, text = %q", numeric, text)
	}
	for _, invalid := range []json.RawMessage{nil, json.RawMessage(`""`), json.RawMessage(`null`)} {
		if got := appServerRequestKey(invalid); got != "" {
			t.Fatalf("invalid id %s = %q", invalid, got)
		}
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

func TestAnswerAppServerRequestDeclinesMutationInPlan(t *testing.T) {
	var encoded strings.Builder
	err := newAppServerRequestHandler(
		agent.RunRequest{Mode: agent.RunModePlan},
		func(message any) error {
			data, marshalErr := json.Marshal(message)
			if marshalErr == nil {
				encoded.Write(data)
			}
			return marshalErr
		},
	).Answer(context.Background(), appServerEnvelope{
		ID:     []byte("42"),
		Method: "item/fileChange/requestApproval",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), `"decision":"decline"`) {
		t.Fatalf("response = %s", encoded.String())
	}
}

func TestAnswerAppServerUserInputWaitsForCorrelatedAnswers(t *testing.T) {
	var captured agent.InteractionRequest
	var encoded strings.Builder
	handler := newAppServerRequestHandler(
		agent.RunRequest{
			Interactions: testInteractionHandlerFunc(func(
				_ context.Context,
				request agent.InteractionRequest,
			) (agent.PendingInteraction, error) {
				captured = request
				return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
					return agent.InteractionResponse{
						Answers: map[string][]string{"scope": {"Backend", "Frontend"}},
					}, nil
				}), nil
			}),
		},
		func(message any) error {
			data, err := json.Marshal(message)
			if err == nil {
				encoded.Write(data)
			}
			return err
		},
	)

	err := handler.Answer(context.Background(), appServerEnvelope{
		ID:     json.RawMessage(`42`),
		Method: "item/tool/requestUserInput",
		Params: json.RawMessage(`{
			"itemId":"question-item",
			"isBlocking":false,
			"autoResolutionMs":60000,
			"questions":[{
				"id":"scope",
				"header":"Scope",
				"question":"Which layers?",
				"isOther":true,
				"options":[{"label":"Backend"},{"label":"Frontend"}]
			}]
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.ID != "question-item" || captured.Kind != agent.InteractionUserInput ||
		captured.ToolName != "AskUserQuestion" || captured.Blocking ||
		captured.AutoResolutionMS != nonBlockingUserInputAutoResolutionMS {
		t.Fatalf("interaction = %#v", captured)
	}
	if !strings.Contains(string(captured.Input), `"id":"scope"`) {
		t.Fatalf("interaction input = %s", captured.Input)
	}
	if !strings.Contains(encoded.String(), `"scope":{"answers":["Backend","Frontend"]}`) {
		t.Fatalf("response = %s", encoded.String())
	}
}

func TestAnswerAppServerUserInputFailsWithoutInteractionHandler(t *testing.T) {
	handler := newAppServerRequestHandler(
		agent.RunRequest{},
		func(any) error { return nil },
	)
	err := handler.Answer(context.Background(), appServerEnvelope{
		ID:     json.RawMessage(`42`),
		Method: "tool/requestUserInput",
		Params: json.RawMessage(`{
			"itemId":"question-item",
			"questions":[{
				"id":"scope","question":"Which layer?","isOther":true,
				"options":[{"label":"Backend"}]
			}]
		}`),
	})
	if err == nil || !strings.Contains(err.Error(), "no interaction handler") {
		t.Fatalf("error = %v", err)
	}
}

func TestAnswerAppServerUserInputRejectsEmptyQuestions(t *testing.T) {
	handler := newAppServerRequestHandler(
		agent.RunRequest{Interactions: testInteractionHandlerFunc(func(
			context.Context,
			agent.InteractionRequest,
		) (agent.PendingInteraction, error) {
			return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
				return agent.InteractionResponse{}, nil
			}), nil
		})},
		func(any) error { return nil },
	)
	err := handler.Answer(context.Background(), appServerEnvelope{
		ID:     json.RawMessage(`42`),
		Method: "tool/requestUserInput",
		Params: json.RawMessage(`{"itemId":"question-item","questions":[]}`),
	})
	if err == nil || !strings.Contains(err.Error(), "no questions") {
		t.Fatalf("error = %v", err)
	}
}

func TestAnswerAppServerUserInputPreservesEmptyOuterAnswersOnAutoResolution(t *testing.T) {
	var encoded strings.Builder
	handler := newAppServerRequestHandler(
		agent.RunRequest{
			Interactions: testInteractionHandlerFunc(func(
				_ context.Context,
				request agent.InteractionRequest,
			) (agent.PendingInteraction, error) {
				if request.Blocking || request.AutoResolutionMS != nonBlockingUserInputAutoResolutionMS {
					t.Fatalf("interaction = %#v", request)
				}
				return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
					return agent.InteractionResponse{Answers: map[string][]string{}}, nil
				}), nil
			}),
		},
		func(message any) error {
			data, err := json.Marshal(message)
			if err == nil {
				encoded.Write(data)
			}
			return err
		},
	)
	err := handler.Answer(context.Background(), appServerEnvelope{
		ID:     json.RawMessage(`42`),
		Method: "item/tool/requestUserInput",
		Params: json.RawMessage(`{
			"itemId":"question-item",
			"isBlocking":false,
			"autoResolutionMs":null,
			"questions":[{"id":"scope","question":"Which scope?","isOther":false}]
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), `"answers":{}`) || strings.Contains(encoded.String(), `"scope"`) {
		t.Fatalf("auto-resolution response = %s", encoded.String())
	}
}

func TestAnswerAppServerBlockingSecretIgnoresDeprecatedTimeout(t *testing.T) {
	var captured agent.InteractionRequest
	handler := newAppServerRequestHandler(
		agent.RunRequest{
			Interactions: testInteractionHandlerFunc(func(
				_ context.Context,
				request agent.InteractionRequest,
			) (agent.PendingInteraction, error) {
				captured = request
				return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
					return agent.InteractionResponse{Answers: map[string][]string{"token": {"secret"}}}, nil
				}), nil
			}),
		},
		func(any) error { return nil },
	)
	err := handler.Answer(context.Background(), appServerEnvelope{
		ID:     json.RawMessage(`42`),
		Method: "item/tool/requestUserInput",
		Params: json.RawMessage(`{
			"itemId":"question-item",
			"isBlocking":true,
			"autoResolutionMs":1,
			"questions":[{
				"id":"token","question":"Token?","isOther":true,"isSecret":true,"options":null
			}]
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !captured.Blocking || !captured.Sensitive || captured.AutoResolutionMS != 0 {
		t.Fatalf("interaction = %#v", captured)
	}
	if !strings.Contains(string(captured.Input), `"isSecret":true`) ||
		!strings.Contains(string(captured.Input), `"isOther":true`) {
		t.Fatalf("interaction input = %s", captured.Input)
	}
}

func TestAnswerAppServerPrefixesNativeFreeformAnswers(t *testing.T) {
	var encoded strings.Builder
	handler := newAppServerRequestHandler(
		agent.RunRequest{
			Interactions: testInteractionHandlerFunc(func(
				_ context.Context,
				_ agent.InteractionRequest,
			) (agent.PendingInteraction, error) {
				return testPendingInteractionFunc(func() (agent.InteractionResponse, error) {
					return agent.InteractionResponse{Answers: map[string][]string{
						"scope": {"Backend", "A custom layer"},
					}}, nil
				}), nil
			}),
		},
		func(message any) error {
			data, err := json.Marshal(message)
			if err == nil {
				encoded.Write(data)
			}
			return err
		},
	)
	err := handler.Answer(context.Background(), appServerEnvelope{
		ID:     json.RawMessage(`42`),
		Method: "item/tool/requestUserInput",
		Params: json.RawMessage(`{
			"itemId":"question-item",
			"questions":[{
				"id":"scope","question":"Which layer?","isOther":true,
				"options":[{"label":"Backend"}]
			}]
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), `"answers":["Backend","user_note: A custom layer"]`) {
		t.Fatalf("response = %s", encoded.String())
	}
}

func TestCodexUserInputAnswersPreservesOptionNotesWhenOtherIsDisabled(t *testing.T) {
	question := appServerUserQuestion{
		IsOther: false,
		Options: []appServerQuestionOption{{Label: "Backend"}},
	}
	got := codexUserInputAnswers(question, []string{"Backend", "Custom"})
	if len(got) != 2 || got[0] != "Backend" || got[1] != "user_note: Custom" {
		t.Fatalf("answers = %#v", got)
	}

	freeform := codexUserInputAnswers(appServerUserQuestion{}, []string{"Custom"})
	if len(freeform) != 1 || freeform[0] != "user_note: Custom" {
		t.Fatalf("freeform answers = %#v", freeform)
	}
}
