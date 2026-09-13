package lifecycle

import (
	"context"
	"sync"
	"sync/atomic"
)

// UpdatePhase describes where a durable update attempt is in its lifecycle.
type UpdatePhase string

const (
	UpdateStarted   UpdatePhase = "started"
	UpdateCompleted UpdatePhase = "completed"
	UpdateFailed    UpdatePhase = "failed"
)

// UpdateEvent is the non-sensitive lifecycle identity delivered to
// subscribers. AttemptID correlates a start event with exactly one terminal
// event within this process. Context is passed through for request-scoped
// tracing; subscribers must not retain it beyond their event handling.
type UpdateEvent struct {
	Context   context.Context
	AttemptID uint64
	Phase     UpdatePhase
	Source    string
	Operation string
	Subject   string
	Err       error
}

// UpdatePublisher is the producer-facing capability for durable update
// attempts. It contains no subscriber registration API, so producers cannot
// depend on or control consumers.
type UpdatePublisher interface {
	BeginUpdate(ctx context.Context, source, operation, subject string) (finish func(error))
}

type updatePublisher struct {
	manager       *Manager
	nextAttemptID atomic.Uint64
}

// BeginUpdate publishes a started event and returns the operation's terminal
// publisher. Calling finish with nil publishes completed; a non-nil error
// publishes failed with that exact error. Only the first call has an effect.
func (p *updatePublisher) BeginUpdate(ctx context.Context, source, operation, subject string) (finish func(error)) {
	attemptID := p.nextAttemptID.Add(1)
	event := UpdateEvent{
		Context:   ctx,
		AttemptID: attemptID,
		Phase:     UpdateStarted,
		Source:    source,
		Operation: operation,
		Subject:   subject,
	}
	p.manager.publishUpdate(event)

	var once sync.Once
	return func(err error) {
		once.Do(func() {
			event.Err = err
			if err == nil {
				event.Phase = UpdateCompleted
			} else {
				event.Phase = UpdateFailed
			}
			p.manager.publishUpdate(event)
		})
	}
}
