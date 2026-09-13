// Package publishers contains the application lifecycle publishers registered
// by lifecycle.Manager.
package publishers

import (
	"context"
	"sync"
)

// UpdateStartedEvent describes an application self-update that has started.
type UpdateStartedEvent struct {
	Target    string
	Kind      string
	StartedBy string
}

// UpdateSucceededEvent describes an application self-update that completed
// successfully. It is intentionally the same small, non-sensitive identity as
// the start event; subscribers that need durable details own their own state.
type UpdateSucceededEvent struct {
	Target    string
	Kind      string
	StartedBy string
}

// UpdateFailedEvent describes an application self-update that terminated
// unsuccessfully.
type UpdateFailedEvent struct {
	Target    string
	Kind      string
	StartedBy string
}

// CoreSubscriber receives events owned by the application's core lifecycle.
type CoreSubscriber interface {
	OnUpdateStarted(context.Context, UpdateStartedEvent)
	OnUpdateSucceeded(context.Context, UpdateSucceededEvent)
	OnUpdateFailed(context.Context, UpdateFailedEvent)
}

type coreSubscription struct {
	id         uint64
	subscriber CoreSubscriber
}

// Core publishes lifecycle events for the remote.futrx application itself.
// It owns its subscribers; lifecycle.Manager only registers the publisher.
type Core struct {
	mu            sync.RWMutex
	nextID        uint64
	subscriptions []coreSubscription
}

// NewCore creates a core publisher with no subscribers.
func NewCore() *Core {
	return &Core{}
}

// Subscribe registers a core lifecycle subscriber and returns an idempotent
// function that removes it.
func (c *Core) Subscribe(subscriber CoreSubscriber) (unsubscribe func()) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.subscriptions = append(c.subscriptions, coreSubscription{id: id, subscriber: subscriber})
	c.mu.Unlock()

	removed := false
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if removed {
			return
		}
		removed = true
		for index, subscription := range c.subscriptions {
			if subscription.id == id {
				c.subscriptions = append(c.subscriptions[:index], c.subscriptions[index+1:]...)
				return
			}
		}
	}
}

// PublishUpdateStarted notifies subscribers that an application update has
// started. Subscribers run synchronously in registration order and must return
// promptly.
func (c *Core) PublishUpdateStarted(ctx context.Context, target, kind, startedBy string) {
	event := UpdateStartedEvent{Target: target, Kind: kind, StartedBy: startedBy}
	for _, subscription := range c.snapshot() {
		subscription.subscriber.OnUpdateStarted(ctx, event)
	}
}

// PublishUpdateSucceeded notifies subscribers that an application update
// completed successfully.
func (c *Core) PublishUpdateSucceeded(ctx context.Context, target, kind, startedBy string) {
	event := UpdateSucceededEvent{Target: target, Kind: kind, StartedBy: startedBy}
	for _, subscription := range c.snapshot() {
		subscription.subscriber.OnUpdateSucceeded(ctx, event)
	}
}

// PublishUpdateFailed notifies subscribers that an application update
// terminated unsuccessfully.
func (c *Core) PublishUpdateFailed(ctx context.Context, target, kind, startedBy string) {
	event := UpdateFailedEvent{Target: target, Kind: kind, StartedBy: startedBy}
	for _, subscription := range c.snapshot() {
		subscription.subscriber.OnUpdateFailed(ctx, event)
	}
}

func (c *Core) snapshot() []coreSubscription {
	c.mu.RLock()
	defer c.mu.RUnlock()
	subscriptions := make([]coreSubscription, len(c.subscriptions))
	copy(subscriptions, c.subscriptions)
	return subscriptions
}
