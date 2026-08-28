package prompt

import (
	"context"
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

// interactionBroker keeps a running provider request correlated with a later
// WebSocket response. Pending requests are scoped to a chat so provider item
// IDs never collide across conversations.
type interactionBroker struct {
	mu      sync.Mutex
	pending map[servicechat.ID]map[string]*pendingInteractionSession
}

func newInteractionBroker() *interactionBroker {
	return &interactionBroker{
		pending: make(map[servicechat.ID]map[string]*pendingInteractionSession),
	}
}

func (broker *interactionBroker) register(
	chatID servicechat.ID,
	interactionID string,
	ctx context.Context,
	deadline time.Time,
) (*pendingInteractionSession, error) {
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return nil, ErrInvalidInteractionID
	}

	broker.mu.Lock()
	defer broker.mu.Unlock()
	byID := broker.pending[chatID]
	if byID == nil {
		byID = make(map[string]*pendingInteractionSession)
		broker.pending[chatID] = byID
	}
	if _, exists := byID[interactionID]; exists {
		return nil, ErrInteractionPending
	}
	pending := newPendingInteractionSession(broker, chatID, interactionID, ctx, deadline)
	byID[interactionID] = pending
	return pending, nil
}

// complete elects exactly one terminal outcome under the broker lock. The
// waiter then consumes that winner from a buffered channel; cancellation,
// timeout, and a WebSocket answer can never all report success.
func (broker *interactionBroker) complete(
	pending *pendingInteractionSession,
	result interactionResult,
) bool {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	return broker.completeLocked(pending, result)
}

func (broker *interactionBroker) completeLocked(
	pending *pendingInteractionSession,
	result interactionResult,
) bool {
	byID := broker.pending[pending.chatID]
	if byID == nil || byID[pending.interactionID] != pending {
		return false
	}
	delete(byID, pending.interactionID)
	if len(byID) == 0 {
		delete(broker.pending, pending.chatID)
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
		broker.completeLocked(pending, interactionResult{
			err:        err,
			resolution: interactionCancelled,
		})
		return false
	}
	if !pending.autoSnoozed && !pending.deadline.IsZero() && !time.Now().Before(pending.deadline) {
		broker.completeLocked(pending, interactionResult{
			response:   agent.InteractionResponse{Answers: map[string][]string{}},
			resolution: interactionAutoResolved,
		})
		return false
	}
	return broker.completeLocked(pending, interactionResult{
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
		broker.completeLocked(pending, interactionResult{
			err:        err,
			resolution: interactionCancelled,
		})
		return false
	}
	if !time.Now().Before(pending.deadline) {
		broker.completeLocked(pending, interactionResult{
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
	pending *pendingInteractionSession,
) bool {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	byID := broker.pending[pending.chatID]
	if byID == nil || byID[pending.interactionID] != pending || pending.autoSnoozed {
		return false
	}
	if err := pending.ctx.Err(); err != nil {
		return broker.completeLocked(pending, interactionResult{
			err:        err,
			resolution: interactionCancelled,
		})
	}
	return broker.completeLocked(pending, interactionResult{
		response:   agent.InteractionResponse{Answers: map[string][]string{}},
		resolution: interactionAutoResolved,
	})
}

// runInteractionHandler binds the application-owned broker to one active chat
// run. BeginInteraction returns only after the request event is emitted, which
// is the ordering boundary required by asynchronous provider transports.
type runInteractionHandler struct {
	service *Service
	chatID  servicechat.ID
	emit    func(ChatEvent)
}

func (handler runInteractionHandler) BeginInteraction(
	ctx context.Context,
	request agent.InteractionRequest,
) (agent.PendingInteraction, error) {
	return handler.service.beginInteraction(ctx, handler.chatID, request, handler.emit)
}

func (rnr *Service) beginInteraction(
	ctx context.Context,
	chatID servicechat.ID,
	request agent.InteractionRequest,
	emit func(ChatEvent),
) (agent.PendingInteraction, error) {
	request.ID = strings.TrimSpace(request.ID)
	var deadline time.Time
	if !request.Blocking && request.AutoResolutionMS > 0 {
		deadline = time.Now().Add(time.Duration(request.AutoResolutionMS) * time.Millisecond)
	}
	pending, err := rnr.interactions.register(chatID, request.ID, ctx, deadline)
	if err != nil {
		return nil, err
	}

	emit(ChatEvent{
		T:        time.Now().UnixMilli(),
		Type:     "interaction_request",
		ID:       request.ID,
		ToolName: request.ToolName,
		Input:    request.Input,
	})
	return newInteractionSession(pending, request, emit), nil
}

var _ agent.InteractionHandler = runInteractionHandler{}

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
