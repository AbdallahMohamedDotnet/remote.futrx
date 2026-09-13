package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type contextKey string

const testContextKey contextKey = "test"

func TestManagerIntroducesOneStableUpdatePublisher(t *testing.T) {
	manager := NewManager()
	first := manager.Publishers().Updates
	second := manager.Publishers().Updates
	if first == nil {
		t.Fatal("manager returned a nil update publisher")
	}
	if first != second {
		t.Fatal("manager returned different update publisher instances")
	}
}

func TestSubscriberReceivesCorrelatedStartedAndCompletedEvents(t *testing.T) {
	manager := NewManager()
	subscription := manager.SubscribeUpdates()
	defer subscription.Close()

	finish := manager.Publishers().Updates.BeginUpdate(context.Background(), "src", "op", "subject")
	finish(nil)

	started := <-subscription.Events()
	completed := <-subscription.Events()
	if started.Phase != UpdateStarted || completed.Phase != UpdateCompleted {
		t.Fatalf("phases = %q, %q; want %q, %q", started.Phase, completed.Phase, UpdateStarted, UpdateCompleted)
	}
	if started.AttemptID == 0 || completed.AttemptID != started.AttemptID {
		t.Fatalf("attempt IDs = %d, %d; want the same non-zero ID", started.AttemptID, completed.AttemptID)
	}
	if started.Source != "src" || started.Operation != "op" || started.Subject != "subject" {
		t.Fatalf("started event = %+v", started)
	}
	if completed.Err != nil {
		t.Fatalf("completed error = %v, want nil", completed.Err)
	}
}

func TestSubscriberReceivesExactContextAndFailure(t *testing.T) {
	manager := NewManager()
	subscription := manager.SubscribeUpdates()
	defer subscription.Close()

	ctx := context.WithValue(context.Background(), testContextKey, "value")
	failure := errors.New("specific failure")
	finish := manager.Publishers().Updates.BeginUpdate(ctx, "two-factor", "disable", "user@example.com")
	finish(failure)

	started := <-subscription.Events()
	failed := <-subscription.Events()
	if started.Context.Value(testContextKey) != "value" || failed.Context.Value(testContextKey) != "value" {
		t.Fatal("subscriber did not receive the supplied context")
	}
	if failed.Phase != UpdateFailed {
		t.Fatalf("phase = %q, want %q", failed.Phase, UpdateFailed)
	}
	if !errors.Is(failed.Err, failure) {
		t.Fatalf("error = %v, want %v", failed.Err, failure)
	}
}

func TestEverySubscriberReceivesTheSameEventSequence(t *testing.T) {
	manager := NewManager()
	first := manager.SubscribeUpdates()
	second := manager.SubscribeUpdates()
	defer first.Close()
	defer second.Close()

	finish := manager.Publishers().Updates.BeginUpdate(context.Background(), "src", "op", "subject")
	finish(nil)

	for _, subscription := range []*UpdateSubscription{first, second} {
		started := <-subscription.Events()
		completed := <-subscription.Events()
		if started.Phase != UpdateStarted || completed.Phase != UpdateCompleted || started.AttemptID != completed.AttemptID {
			t.Fatalf("unexpected event sequence: %+v, %+v", started, completed)
		}
	}
}

func TestFinishingAnUpdateMoreThanOncePublishesOneTerminalEvent(t *testing.T) {
	manager := NewManager()
	subscription := manager.SubscribeUpdates()
	defer subscription.Close()

	finish := manager.Publishers().Updates.BeginUpdate(context.Background(), "src", "op", "subject")
	finish(nil)
	finish(errors.New("late failure"))

	<-subscription.Events()
	terminal := <-subscription.Events()
	if terminal.Phase != UpdateCompleted {
		t.Fatalf("terminal phase = %q, want %q", terminal.Phase, UpdateCompleted)
	}
	select {
	case extra := <-subscription.Events():
		t.Fatalf("unexpected extra terminal event: %+v", extra)
	default:
	}
}

func TestClosedSubscriberReceivesNoLaterEvents(t *testing.T) {
	manager := NewManager()
	subscription := manager.SubscribeUpdates()
	subscription.Close()
	subscription.Close()

	finish := manager.Publishers().Updates.BeginUpdate(context.Background(), "src", "op", "subject")
	finish(nil)

	if _, open := <-subscription.Events(); open {
		t.Fatal("closed subscription channel remained open")
	}
}

func TestSlowSubscriberIsRemovedInsteadOfBlockingPublisher(t *testing.T) {
	manager := NewManager()
	subscription := manager.SubscribeUpdates()

	for i := 0; i <= updateSubscriptionBuffer; i++ {
		manager.Publishers().Updates.BeginUpdate(context.Background(), "src", "op", "subject")
	}

	count := 0
	for range subscription.Events() {
		count++
	}
	if count != updateSubscriptionBuffer {
		t.Fatalf("buffered events = %d, want %d", count, updateSubscriptionBuffer)
	}
}

func TestConcurrentUpdatesHaveDistinctAttemptIDs(t *testing.T) {
	manager := NewManager()
	subscription := manager.SubscribeUpdates()
	defer subscription.Close()
	const attempts = 100

	var wait sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			manager.Publishers().Updates.BeginUpdate(context.Background(), "src", "op", "subject")
		}()
	}
	wait.Wait()

	seen := make(map[uint64]struct{}, attempts)
	for i := 0; i < attempts; i++ {
		event := <-subscription.Events()
		if event.AttemptID == 0 {
			t.Fatal("received zero attempt ID")
		}
		if _, duplicate := seen[event.AttemptID]; duplicate {
			t.Fatalf("received duplicate attempt ID %d", event.AttemptID)
		}
		seen[event.AttemptID] = struct{}{}
	}
}

func TestConcurrentSubscriptionAndPublicationIsSafe(t *testing.T) {
	manager := NewManager()
	publisher := manager.Publishers().Updates
	var wait sync.WaitGroup

	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for j := 0; j < 20; j++ {
				subscription := manager.SubscribeUpdates()
				finish := publisher.BeginUpdate(context.Background(), "src", "op", "subject")
				finish(nil)
				subscription.Close()
			}
		}()
	}
	wait.Wait()
}
