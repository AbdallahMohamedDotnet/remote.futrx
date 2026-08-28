package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

var (
	ErrInvalidInteractionID = errors.New("agent interaction is missing an id")
	ErrInteractionPending   = errors.New("agent interaction id is already pending")
)

type pendingInteraction struct {
	ctx         context.Context
	deadline    time.Time
	autoSnoozed bool
	activity    chan struct{}
	result      chan interactionResult
}

type interactionResolution uint8

const (
	interactionAnswered interactionResolution = iota
	interactionAutoResolved
	interactionCancelled
)

type interactionResult struct {
	response   agent.InteractionResponse
	err        error
	resolution interactionResolution
}

// interactionBroker keeps a running provider request correlated with a later
// WebSocket response. Pending requests are scoped to a chat so provider item
// IDs never collide across conversations.
type interactionBroker struct {
	mu      sync.Mutex
	pending map[servicechat.ID]map[string]*pendingInteraction
}

func newInteractionBroker() *interactionBroker {
	return &interactionBroker{
		pending: make(map[servicechat.ID]map[string]*pendingInteraction),
	}
}

func (broker *interactionBroker) register(
	chatID servicechat.ID,
	interactionID string,
	ctx context.Context,
	deadline time.Time,
) (*pendingInteraction, error) {
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return nil, ErrInvalidInteractionID
	}

	broker.mu.Lock()
	defer broker.mu.Unlock()
	byID := broker.pending[chatID]
	if byID == nil {
		byID = make(map[string]*pendingInteraction)
		broker.pending[chatID] = byID
	}
	if _, exists := byID[interactionID]; exists {
		return nil, ErrInteractionPending
	}
	pending := &pendingInteraction{
		ctx:      ctx,
		deadline: deadline,
		activity: make(chan struct{}),
		result:   make(chan interactionResult, 1),
	}
	byID[interactionID] = pending
	return pending, nil
}

// complete elects exactly one terminal outcome under the broker lock. The
// waiter then consumes that winner from a buffered channel; cancellation,
// timeout, and a WebSocket answer can never all report success.
func (broker *interactionBroker) complete(
	chatID servicechat.ID,
	interactionID string,
	pending *pendingInteraction,
	result interactionResult,
) bool {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	return broker.completeLocked(chatID, interactionID, pending, result)
}

func (broker *interactionBroker) completeLocked(
	chatID servicechat.ID,
	interactionID string,
	pending *pendingInteraction,
	result interactionResult,
) bool {
	byID := broker.pending[chatID]
	if byID == nil || byID[interactionID] != pending {
		return false
	}
	delete(byID, interactionID)
	if len(byID) == 0 {
		delete(broker.pending, chatID)
	}
	pending.result <- result
	return true
}

func (broker *interactionBroker) resolve(
	chatID servicechat.ID,
	interactionID string,
	response agent.InteractionResponse,
) bool {
	interactionID = strings.TrimSpace(interactionID)
	broker.mu.Lock()
	defer broker.mu.Unlock()
	pending := broker.pending[chatID][interactionID]
	if pending == nil {
		return false
	}
	if err := pending.ctx.Err(); err != nil {
		broker.completeLocked(chatID, interactionID, pending, interactionResult{
			err:        err,
			resolution: interactionCancelled,
		})
		return false
	}
	if !pending.autoSnoozed && !pending.deadline.IsZero() && !time.Now().Before(pending.deadline) {
		broker.completeLocked(chatID, interactionID, pending, interactionResult{
			response:   agent.InteractionResponse{Answers: map[string][]string{}},
			resolution: interactionAutoResolved,
		})
		return false
	}
	return broker.completeLocked(chatID, interactionID, pending, interactionResult{
		response:   response,
		resolution: interactionAnswered,
	})
}

// snoozeAutoResolution mirrors Codex's native TUI behavior: once a person has
// started interacting with a non-blocking question, its automatic empty
// response is disabled. The deadline check and snooze happen under the same
// lock as response resolution, so activity cannot revive an expired request.
func (broker *interactionBroker) snoozeAutoResolution(
	chatID servicechat.ID,
	interactionID string,
) bool {
	interactionID = strings.TrimSpace(interactionID)
	broker.mu.Lock()
	defer broker.mu.Unlock()
	pending := broker.pending[chatID][interactionID]
	if pending == nil || pending.autoSnoozed || pending.deadline.IsZero() {
		return pending != nil
	}
	if err := pending.ctx.Err(); err != nil {
		broker.completeLocked(chatID, interactionID, pending, interactionResult{
			err:        err,
			resolution: interactionCancelled,
		})
		return false
	}
	if !time.Now().Before(pending.deadline) {
		broker.completeLocked(chatID, interactionID, pending, interactionResult{
			response:   agent.InteractionResponse{Answers: map[string][]string{}},
			resolution: interactionAutoResolved,
		})
		return false
	}
	pending.autoSnoozed = true
	close(pending.activity)
	return true
}

func (broker *interactionBroker) expireAutoResolution(
	chatID servicechat.ID,
	interactionID string,
	pending *pendingInteraction,
) bool {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	byID := broker.pending[chatID]
	if byID == nil || byID[interactionID] != pending || pending.autoSnoozed {
		return false
	}
	if err := pending.ctx.Err(); err != nil {
		return broker.completeLocked(chatID, interactionID, pending, interactionResult{
			err:        err,
			resolution: interactionCancelled,
		})
	}
	return broker.completeLocked(chatID, interactionID, pending, interactionResult{
		response:   agent.InteractionResponse{Answers: map[string][]string{}},
		resolution: interactionAutoResolved,
	})
}

func (rnr *Service) requestInteraction(
	ctx context.Context,
	chatID servicechat.ID,
	request agent.InteractionRequest,
	emit func(ChatEvent),
) (agent.InteractionResponse, error) {
	request.ID = strings.TrimSpace(request.ID)
	var deadline time.Time
	if !request.Blocking && request.AutoResolutionMS > 0 {
		deadline = time.Now().Add(time.Duration(request.AutoResolutionMS) * time.Millisecond)
	}
	pending, err := rnr.interactions.register(chatID, request.ID, ctx, deadline)
	if err != nil {
		return agent.InteractionResponse{}, err
	}

	emit(ChatEvent{
		T:        time.Now().UnixMilli(),
		Type:     "interaction_request",
		ID:       request.ID,
		ToolName: request.ToolName,
		Input:    request.Input,
	})
	if request.Registered != nil {
		request.Registered()
	}
	var autoResolve <-chan time.Time
	var timer *time.Timer
	if !deadline.IsZero() {
		delay := time.Until(deadline)
		timer = time.NewTimer(delay)
		autoResolve = timer.C
		defer timer.Stop()
	}

	activity := (<-chan struct{})(pending.activity)
	var result interactionResult
	for {
		select {
		case result = <-pending.result:
			goto resolved
		case <-activity:
			if timer != nil {
				timer.Stop()
			}
			autoResolve = nil
			activity = nil
		case <-autoResolve:
			if rnr.interactions.expireAutoResolution(chatID, request.ID, pending) {
				result = <-pending.result
				goto resolved
			}
			autoResolve = nil
		case <-ctx.Done():
			rnr.interactions.complete(chatID, request.ID, pending, interactionResult{
				err:        ctx.Err(),
				resolution: interactionCancelled,
			})
			result = <-pending.result
			goto resolved
		}
	}

resolved:
	switch result.resolution {
	case interactionAnswered:
		output := "Secret response received"
		if !request.Sensitive {
			encoded, _ := json.Marshal(result.response)
			output = string(encoded)
		}
		emit(ChatEvent{
			T:      time.Now().UnixMilli(),
			Type:   "interaction_resolved",
			ID:     request.ID,
			Output: output,
		})
		return result.response, result.err
	case interactionAutoResolved:
		emit(ChatEvent{
			T:      time.Now().UnixMilli(),
			Type:   "interaction_resolved",
			ID:     request.ID,
			Output: "No response before the agent continued",
		})
		return result.response, result.err
	default:
		emit(ChatEvent{
			T:       time.Now().UnixMilli(),
			Type:    "interaction_resolved",
			ID:      request.ID,
			Output:  "Agent interaction cancelled",
			IsError: true,
		})
		return result.response, result.err
	}
}

// RespondInteraction resumes a provider request. It returns false
// when the request is no longer pending, including after cancellation or a
// backend restart.
func (rnr *Service) RespondInteraction(
	chatID servicechat.ID,
	interactionID string,
	response agent.InteractionResponse,
) bool {
	if rnr == nil || rnr.interactions == nil {
		return false
	}
	return rnr.interactions.resolve(chatID, interactionID, response)
}

// SnoozeInteractionAutoResolution records real user activity for a native
// non-blocking request. It is intentionally transient and contains no answer.
func (rnr *Service) SnoozeInteractionAutoResolution(
	chatID servicechat.ID,
	interactionID string,
) bool {
	if rnr == nil || rnr.interactions == nil {
		return false
	}
	return rnr.interactions.snoozeAutoResolution(chatID, interactionID)
}
