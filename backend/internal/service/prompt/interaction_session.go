package prompt

import (
	"context"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

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

// pendingInteractionSession owns the process-local wait state for one native
// provider request. The broker owns registration and terminal election; this
// session owns waiting, timer lifecycle, activity, and cancellation handling.
type pendingInteractionSession struct {
	broker        *interactionBroker
	chatID        servicechat.ID
	interactionID string
	ctx           context.Context
	deadline      time.Time
	autoSnoozed   bool
	activity      chan struct{}
	result        chan interactionResult
	timer         *time.Timer
}

func newPendingInteractionSession(
	broker *interactionBroker,
	chatID servicechat.ID,
	interactionID string,
	ctx context.Context,
	deadline time.Time,
) *pendingInteractionSession {
	return &pendingInteractionSession{
		broker:        broker,
		chatID:        chatID,
		interactionID: interactionID,
		ctx:           ctx,
		deadline:      deadline,
		activity:      make(chan struct{}),
		result:        make(chan interactionResult, 1),
	}
}

// awaitResult blocks until the broker elects one terminal outcome. Timer
// cleanup is deliberately separate so the service can emit the terminal card
// before its deferred cleanup runs, preserving the existing lifecycle order.
func (session *pendingInteractionSession) awaitResult() interactionResult {
	var autoResolve <-chan time.Time
	if !session.deadline.IsZero() {
		delay := time.Until(session.deadline)
		session.timer = time.NewTimer(delay)
		autoResolve = session.timer.C
	}

	activity := (<-chan struct{})(session.activity)
	for {
		select {
		case result := <-session.result:
			return result
		case <-activity:
			if session.timer != nil {
				session.timer.Stop()
			}
			autoResolve = nil
			activity = nil
		case <-autoResolve:
			if session.broker.expireAutoResolution(session) {
				return <-session.result
			}
			autoResolve = nil
		case <-session.ctx.Done():
			session.broker.complete(session, interactionResult{
				err:        session.ctx.Err(),
				resolution: interactionCancelled,
			})
			return <-session.result
		}
	}
}

func (session *pendingInteractionSession) finishWaiting() {
	if session.timer != nil {
		session.timer.Stop()
	}
}
