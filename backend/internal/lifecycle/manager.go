// Package lifecycle coordinates application-wide lifecycle publishers and
// subscriptions. The process composition root owns one Manager and gives
// producers only the narrow publisher they need.
package lifecycle

import "sync"

const updateSubscriptionBuffer = 256

// Publishers is the set of publishing capabilities introduced by Manager.
// Add future lifecycle publishers here so construction remains centralized.
type Publishers struct {
	Updates UpdatePublisher
}

// Manager owns lifecycle publishers and their subscriber registries.
// Publishing never invokes subscriber code: each subscription receives a
// buffered event stream and is removed if it cannot keep up.
type Manager struct {
	mu                sync.Mutex
	updateSubscribers map[*UpdateSubscription]struct{}
	updates           *updatePublisher
}

// NewManager creates an empty lifecycle manager and all of its publishers.
// It starts no goroutines and opens no external resources.
func NewManager() *Manager {
	manager := &Manager{
		updateSubscribers: make(map[*UpdateSubscription]struct{}),
	}
	manager.updates = &updatePublisher{manager: manager}
	return manager
}

// Publishers returns the publishing capabilities owned by the manager.
func (m *Manager) Publishers() Publishers {
	return Publishers{Updates: m.updates}
}

// SubscribeUpdates registers a subscriber for update lifecycle events.
// The caller owns the returned subscription and must close it when done.
func (m *Manager) SubscribeUpdates() *UpdateSubscription {
	subscription := &UpdateSubscription{
		manager: m,
		events:  make(chan UpdateEvent, updateSubscriptionBuffer),
	}
	m.mu.Lock()
	m.updateSubscribers[subscription] = struct{}{}
	m.mu.Unlock()
	return subscription
}

func (m *Manager) publishUpdate(event UpdateEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for subscription := range m.updateSubscribers {
		if subscription.trySend(event) {
			continue
		}
		delete(m.updateSubscribers, subscription)
		subscription.close()
	}
}

// UpdateSubscription is one subscriber's ordered stream of update lifecycle
// events. A slow subscriber is closed and removed instead of delaying the
// operation that published the event.
type UpdateSubscription struct {
	manager *Manager
	events  chan UpdateEvent
	mu      sync.Mutex
	closed  bool
}

// Events returns the subscription's read-only event stream.
func (s *UpdateSubscription) Events() <-chan UpdateEvent {
	return s.events
}

// Close removes the subscription. Repeated calls are harmless.
func (s *UpdateSubscription) Close() {
	s.manager.mu.Lock()
	delete(s.manager.updateSubscribers, s)
	s.close()
	s.manager.mu.Unlock()
}

func (s *UpdateSubscription) trySend(event UpdateEvent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.events <- event:
		return true
	default:
		return false
	}
}

func (s *UpdateSubscription) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.events)
}
