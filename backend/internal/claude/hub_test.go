package claude

import (
	"context"
	"testing"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/chat"
)

func TestHubSubscribeReplaysAndBroadcasts(t *testing.T) {
	store, err := chat.NewChatStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(chat.ChatMeta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent("abcd", ChatEvent{T: 1, Type: "user", Text: "hi"}); err != nil {
		t.Fatal(err)
	}

	hub := NewHub(store)
	sub, err := hub.Subscribe("abcd")
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

	hub.Emit("abcd", ChatEvent{T: 2, Type: "assistant_text", Text: "hello"})
	if ev := receiveEvent(t, sub); ev.Type != "assistant_text" || ev.Text != "hello" {
		t.Fatalf("unexpected broadcast event: %#v", ev)
	}
}

func TestHubAllowsOnlyOneRunPerChat(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runID, ok := hub.StartRun("abcd", cancel)
	if !ok {
		t.Fatal("first run should start")
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

func receiveEvent(t *testing.T, sub *Subscription) ChatEvent {
	t.Helper()
	select {
	case ev := <-sub.Events():
		return ev
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return ChatEvent{}
	}
}
