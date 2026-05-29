package runhub

import (
	"context"
	"testing"
	"time"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/stores/filechat"
)

func TestHubSubscribeReplaysAndBroadcasts(t *testing.T) {
	store, err := filechat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{T: 1, Type: "user", Text: "hi"}); err != nil {
		t.Fatal(err)
	}

	hub := New(store)
	sub, err := hub.Subscribe(context.Background(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if ev := receiveEvent(t, sub); ev.Type != "user" || ev.Text != "hi" {
		t.Fatalf("unexpected replay event: %#v", ev)
	}
	if ev := receiveEvent(t, sub); ev.Type != "sync" || ev.Running {
		t.Fatalf("unexpected sync event: %#v", ev)
	}

	hub.Emit("abcd", servicechat.Event{T: 2, Type: "assistant_text", Text: "hello"})
	if ev := receiveEvent(t, sub); ev.Type != "assistant_text" || ev.Text != "hello" {
		t.Fatalf("unexpected broadcast event: %#v", ev)
	}
}

func TestHubAllowsOnlyOneRunPerChat(t *testing.T) {
	hub := New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runID, ok := hub.StartRun("abcd", cancel)
	if !ok {
		t.Fatal("first run should start")
	}
	if !hub.IsRunning("abcd") {
		t.Fatal("expected run to be marked active")
	}
	if _, ok := hub.StartRun("abcd", func() {}); ok {
		t.Fatal("second run should be rejected")
	}
	if !hub.CancelRun("abcd") {
		t.Fatal("expected active run to cancel")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancel was not called")
	}

	hub.FinishRun("abcd", runID)
	if _, ok := hub.StartRun("abcd", func() {}); !ok {
		t.Fatal("new run should start after finish")
	}
}

func receiveEvent(t *testing.T, sub *Subscription) servicechat.Event {
	t.Helper()
	select {
	case ev := <-sub.Events():
		return ev
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return servicechat.Event{}
	}
}
