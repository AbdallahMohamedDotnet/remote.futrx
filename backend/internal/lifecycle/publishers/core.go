// Package publishers contains the application lifecycle publishers exposed by
// lifecycle.Registry.
package publishers

import (
	"context"
	"sync"
)

type updateSubscription struct {
	id         uint64
	subscriber UpdateSubscriber
}

// Core publishes lifecycle events for the remote.futrx application itself.
// It owns its subscribers; lifecycle.Registry only exposes the publisher.
type Core struct {
	mu                  sync.RWMutex
	nextID              uint64
	updateSubscriptions []updateSubscription
}

// NewCore creates a core publisher with no subscribers.
func NewCore() *Core {
	return &Core{}
}

// SubscribeUpdates registers an update lifecycle subscriber and returns an
// idempotent function that removes it.
func (c *Core) SubscribeUpdates(subscriber UpdateSubscriber) (unsubscribe func()) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.updateSubscriptions = append(c.updateSubscriptions, updateSubscription{id: id, subscriber: subscriber})
	c.mu.Unlock()

	removed := false
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if removed {
			return
		}
		removed = true
		for index, subscription := range c.updateSubscriptions {
			if subscription.id == id {
				c.updateSubscriptions = append(c.updateSubscriptions[:index], c.updateSubscriptions[index+1:]...)
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
	for _, subscription := range c.snapshotUpdateSubscriptions() {
		subscription.subscriber.OnUpdateStarted(ctx, event)
	}
}

// PublishUpdateSucceeded notifies subscribers that an application update
// completed successfully.
func (c *Core) PublishUpdateSucceeded(ctx context.Context, target, kind, startedBy string) {
	event := UpdateSucceededEvent{Target: target, Kind: kind, StartedBy: startedBy}
	for _, subscription := range c.snapshotUpdateSubscriptions() {
		subscription.subscriber.OnUpdateSucceeded(ctx, event)
	}
}

// PublishUpdateFailed notifies subscribers that an application update
// terminated unsuccessfully.
func (c *Core) PublishUpdateFailed(ctx context.Context, target, kind, startedBy string) {
	event := UpdateFailedEvent{Target: target, Kind: kind, StartedBy: startedBy}
	for _, subscription := range c.snapshotUpdateSubscriptions() {
		subscription.subscriber.OnUpdateFailed(ctx, event)
	}
}

func (c *Core) snapshotUpdateSubscriptions() []updateSubscription {
	c.mu.RLock()
	defer c.mu.RUnlock()
	subscriptions := make([]updateSubscription, len(c.updateSubscriptions))
	copy(subscriptions, c.updateSubscriptions)
	return subscriptions
}
