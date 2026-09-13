package publishers

import (
	"context"
	"sync"
	"testing"
)

type recordingSubscriber struct {
	name   string
	mu     *sync.Mutex
	events *[]recordedEvent
}

type recordedEvent struct {
	subscriber string
	kind       string
	ctx        context.Context
	target     string
	updateKind string
	startedBy  string
}

func (s recordingSubscriber) OnUpdateStarted(ctx context.Context, event UpdateStartedEvent) {
	s.record(ctx, "started", event.Target, event.Kind, event.StartedBy)
}

func (s recordingSubscriber) OnUpdateSucceeded(ctx context.Context, event UpdateSucceededEvent) {
	s.record(ctx, "succeeded", event.Target, event.Kind, event.StartedBy)
}

func (s recordingSubscriber) OnUpdateFailed(ctx context.Context, event UpdateFailedEvent) {
	s.record(ctx, "failed", event.Target, event.Kind, event.StartedBy)
}

func (s recordingSubscriber) record(ctx context.Context, kind, target, updateKind, startedBy string) {
	s.mu.Lock()
	*s.events = append(*s.events, recordedEvent{
		subscriber: s.name,
		kind:       kind,
		ctx:        ctx,
		target:     target,
		updateKind: updateKind,
		startedBy:  startedBy,
	})
	s.mu.Unlock()
}

func TestCorePublishesUpdateStartedToSubscribersInRegistrationOrder(t *testing.T) {
	core := NewCore()
	var mu sync.Mutex
	var events []recordedEvent
	core.Subscribe(recordingSubscriber{name: "first", mu: &mu, events: &events})
	core.Subscribe(recordingSubscriber{name: "second", mu: &mu, events: &events})

	ctx := context.Background()
	core.PublishUpdateStarted(ctx, "0.4.0", "infrastructure", "admin@example.com")

	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].subscriber != "first" || events[1].subscriber != "second" {
		t.Fatalf("subscriber order = %q, %q", events[0].subscriber, events[1].subscriber)
	}
	for _, event := range events {
		if event.kind != "started" || event.target != "0.4.0" || event.updateKind != "infrastructure" || event.startedBy != "admin@example.com" {
			t.Fatalf("event = %+v, want started 0.4.0 infrastructure admin@example.com", event)
		}
	}
	if events[0].ctx != ctx || events[1].ctx != ctx {
		t.Fatal("subscribers did not receive the publishing context")
	}
}

func TestCorePublishesSucceededAndFailed(t *testing.T) {
	core := NewCore()
	var mu sync.Mutex
	var events []recordedEvent
	core.Subscribe(recordingSubscriber{name: "subscriber", mu: &mu, events: &events})

	ctx := context.Background()
	core.PublishUpdateSucceeded(ctx, "0.4.0", "application", "admin@example.com")
	core.PublishUpdateFailed(ctx, "0.5.0", "infrastructure", "other@example.com")

	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].kind != "succeeded" || events[0].target != "0.4.0" || events[0].updateKind != "application" {
		t.Fatalf("succeeded event = %+v", events[0])
	}
	if events[1].kind != "failed" || events[1].target != "0.5.0" || events[1].updateKind != "infrastructure" {
		t.Fatalf("failed event = %+v", events[1])
	}
	if events[0].ctx != ctx || events[1].ctx != ctx {
		t.Fatal("terminal subscribers did not receive the publishing context")
	}
}

func TestCoreUnsubscribeStopsLaterEvents(t *testing.T) {
	core := NewCore()
	var mu sync.Mutex
	var events []recordedEvent
	unsubscribe := core.Subscribe(recordingSubscriber{mu: &mu, events: &events})

	core.PublishUpdateStarted(context.Background(), "0.4.0", "application", "admin@example.com")
	unsubscribe()
	unsubscribe()
	core.PublishUpdateStarted(context.Background(), "0.4.1", "application", "admin@example.com")

	if len(events) != 1 {
		t.Fatalf("events = %d, want only the event published before unsubscribe", len(events))
	}
}

func TestCoreSubscriberMayChangeSubscriptionsDuringDispatch(t *testing.T) {
	core := NewCore()
	done := make(chan struct{})
	core.Subscribe(coreSubscriberFunc(func(context.Context, UpdateStartedEvent) {
		unsubscribe := core.Subscribe(coreSubscriberFunc(func(context.Context, UpdateStartedEvent) {}))
		unsubscribe()
		close(done)
	}))

	core.PublishUpdateStarted(context.Background(), "0.4.0", "application", "admin@example.com")
	<-done
}

type coreSubscriberFunc func(context.Context, UpdateStartedEvent)

func (subscriber coreSubscriberFunc) OnUpdateStarted(ctx context.Context, event UpdateStartedEvent) {
	subscriber(ctx, event)
}

func (coreSubscriberFunc) OnUpdateSucceeded(context.Context, UpdateSucceededEvent) {}
func (coreSubscriberFunc) OnUpdateFailed(context.Context, UpdateFailedEvent)       {}
