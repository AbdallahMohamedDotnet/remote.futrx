package prompt

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

func TestInteractionRoundTripCorrelatesResponseToPendingRun(t *testing.T) {
	service := &Service{interactions: newInteractionBroker()}
	chatID := servicechat.ID("deadbeef")
	events := make(chan ChatEvent, 2)
	type result struct {
		response agent.InteractionResponse
		err      error
	}
	done := make(chan result, 1)

	go func() {
		response, err := service.requestInteraction(
			context.Background(),
			chatID,
			agent.InteractionRequest{
				ID:       "question-1",
				Kind:     agent.InteractionUserInput,
				ToolName: "AskUserQuestion",
				Input:    []byte(`{"questions":[]}`),
			},
			func(event ChatEvent) { events <- event },
		)
		done <- result{response: response, err: err}
	}()

	requestEvent := awaitInteractionEvent(t, events)
	if requestEvent.Type != "interaction_request" || requestEvent.ID != "question-1" || requestEvent.ToolName != "AskUserQuestion" {
		t.Fatalf("request event = %#v", requestEvent)
	}
	wantAnswers := map[string][]string{"scope": {"Backend"}}
	if !service.RespondInteraction(chatID, "question-1", agent.InteractionResponse{Answers: wantAnswers}) {
		t.Fatal("pending interaction did not accept its response")
	}

	resolvedEvent := awaitInteractionEvent(t, events)
	if resolvedEvent.Type != "interaction_resolved" || resolvedEvent.ID != "question-1" || resolvedEvent.IsError {
		t.Fatalf("resolved event = %#v", resolvedEvent)
	}
	select {
	case got := <-done:
		if got.err != nil || !reflect.DeepEqual(got.response.Answers, wantAnswers) {
			t.Fatalf("interaction result = %#v, %v", got.response, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("interaction did not resume")
	}
	if service.RespondInteraction(chatID, "question-1", agent.InteractionResponse{}) {
		t.Fatal("resolved interaction accepted a second response")
	}
}

func TestInteractionCancellationResolvesPendingCard(t *testing.T) {
	service := &Service{interactions: newInteractionBroker()}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan ChatEvent, 2)
	done := make(chan error, 1)
	go func() {
		_, err := service.requestInteraction(
			ctx,
			"deadbeef",
			agent.InteractionRequest{ID: "question-1", ToolName: "AskUserQuestion"},
			func(event ChatEvent) { events <- event },
		)
		done <- err
	}()
	_ = awaitInteractionEvent(t, events)
	cancel()
	resolved := awaitInteractionEvent(t, events)
	if resolved.Type != "interaction_resolved" || !resolved.IsError {
		t.Fatalf("cancel event = %#v", resolved)
	}
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("cancel error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled interaction did not return")
	}
}

func TestInteractionAutoResolutionDoesNotLeaveProviderBlocked(t *testing.T) {
	service := &Service{interactions: newInteractionBroker()}
	events := make(chan ChatEvent, 2)
	done := make(chan error, 1)
	go func() {
		response, err := service.requestInteraction(
			context.Background(),
			"deadbeef",
			agent.InteractionRequest{
				ID:               "question-1",
				ToolName:         "AskUserQuestion",
				AutoResolutionMS: 20,
			},
			func(event ChatEvent) { events <- event },
		)
		if err == nil && response.Answers == nil {
			err = errors.New("auto-resolved response has nil answers")
		}
		done <- err
	}()
	_ = awaitInteractionEvent(t, events)
	resolved := awaitInteractionEvent(t, events)
	if resolved.Type != "interaction_resolved" || resolved.IsError ||
		resolved.Output != "No response before the agent continued" {
		t.Fatalf("auto-resolved event = %#v", resolved)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("auto-resolved interaction did not return")
	}
	if service.RespondInteraction("deadbeef", "question-1", agent.InteractionResponse{}) {
		t.Fatal("auto-resolved interaction accepted a late response")
	}
}

func TestBlockingInteractionIgnoresAdapterTimeout(t *testing.T) {
	service := &Service{interactions: newInteractionBroker()}
	events := make(chan ChatEvent, 2)
	done := make(chan error, 1)
	go func() {
		_, err := service.requestInteraction(
			context.Background(),
			"deadbeef",
			agent.InteractionRequest{
				ID:               "question-1",
				ToolName:         "AskUserQuestion",
				Blocking:         true,
				AutoResolutionMS: 5,
			},
			func(event ChatEvent) { events <- event },
		)
		done <- err
	}()
	_ = awaitInteractionEvent(t, events)
	time.Sleep(20 * time.Millisecond)
	select {
	case event := <-events:
		t.Fatalf("blocking interaction resolved from timeout: %#v", event)
	default:
	}
	if !service.RespondInteraction("deadbeef", "question-1", agent.InteractionResponse{}) {
		t.Fatal("blocking interaction did not accept response")
	}
	_ = awaitInteractionEvent(t, events)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocking interaction did not return")
	}
}

func TestInteractionActivitySnoozesAutoResolution(t *testing.T) {
	service := &Service{interactions: newInteractionBroker()}
	events := make(chan ChatEvent, 2)
	done := make(chan error, 1)
	go func() {
		_, err := service.requestInteraction(
			context.Background(),
			"deadbeef",
			agent.InteractionRequest{
				ID:               "question-1",
				ToolName:         "AskUserQuestion",
				AutoResolutionMS: 20,
			},
			func(event ChatEvent) { events <- event },
		)
		done <- err
	}()
	_ = awaitInteractionEvent(t, events)
	if !service.SnoozeInteractionAutoResolution("deadbeef", "question-1") {
		t.Fatal("pending interaction did not accept activity")
	}
	time.Sleep(40 * time.Millisecond)
	select {
	case event := <-events:
		t.Fatalf("snoozed interaction auto-resolved: %#v", event)
	default:
	}
	if !service.RespondInteraction("deadbeef", "question-1", agent.InteractionResponse{}) {
		t.Fatal("snoozed interaction did not accept response")
	}
	_ = awaitInteractionEvent(t, events)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("snoozed interaction did not return")
	}
}

func TestInteractionRejectsResponseWhenContextAlreadyCancelled(t *testing.T) {
	service := &Service{interactions: newInteractionBroker()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events := make(chan ChatEvent, 2)
	accepted := true

	_, err := service.requestInteraction(
		ctx,
		"deadbeef",
		agent.InteractionRequest{ID: "question-1", ToolName: "AskUserQuestion"},
		func(event ChatEvent) {
			events <- event
			if event.Type == "interaction_request" {
				accepted = service.RespondInteraction(
					"deadbeef",
					"question-1",
					agent.InteractionResponse{Answers: map[string][]string{"scope": {"Backend"}}},
				)
			}
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if accepted {
		t.Fatal("response was accepted after context cancellation")
	}
	_ = awaitInteractionEvent(t, events)
	resolved := awaitInteractionEvent(t, events)
	if resolved.Type != "interaction_resolved" || !resolved.IsError {
		t.Fatalf("resolved event = %#v", resolved)
	}
}

func TestSensitiveInteractionNeverEmitsAnswerContents(t *testing.T) {
	service := &Service{interactions: newInteractionBroker()}
	events := make(chan ChatEvent, 2)
	done := make(chan error, 1)
	const secret = "do-not-persist-this-token"
	go func() {
		_, err := service.requestInteraction(
			context.Background(),
			"deadbeef",
			agent.InteractionRequest{
				ID:        "question-1",
				ToolName:  "AskUserQuestion",
				Sensitive: true,
			},
			func(event ChatEvent) { events <- event },
		)
		done <- err
	}()
	_ = awaitInteractionEvent(t, events)
	if !service.RespondInteraction("deadbeef", "question-1", agent.InteractionResponse{
		Answers: map[string][]string{"token": {secret}},
	}) {
		t.Fatal("sensitive interaction did not accept response")
	}
	resolved := awaitInteractionEvent(t, events)
	if strings.Contains(resolved.Output, secret) || resolved.Output != "Secret response received" {
		t.Fatalf("sensitive resolved output = %q", resolved.Output)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("sensitive interaction did not return")
	}
}

func awaitInteractionEvent(t *testing.T, events <-chan ChatEvent) ChatEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for interaction event")
		return ChatEvent{}
	}
}
