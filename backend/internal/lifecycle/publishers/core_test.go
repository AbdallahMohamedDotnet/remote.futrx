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
	ctx        context.Context
	event      UpdateStartedEvent
}

func (s recordingSubscriber) OnUpdateStarted(ctx context.Context, event UpdateStartedEvent) {
	s.mu.Lock()
	*s.events = append(*s.events, recordedEvent{subscriber: s.name, ctx: ctx, event: event})
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
	want := UpdateStartedEvent{Target: "0.4.0", Kind: "infrastructure", StartedBy: "admin@example.com"}
	if events[0].event != want || events[1].event != want {
		t.Fatalf("events = %+v, want %+v", events, want)
	}
	if events[0].ctx != ctx || events[1].ctx != ctx {
		t.Fatal("subscribers did not receive the publishing context")
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
