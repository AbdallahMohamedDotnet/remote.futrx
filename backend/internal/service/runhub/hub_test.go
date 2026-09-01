package runhub

import (
	"context"
	"errors"
	"testing"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	"github.com/futrx-com/remote.futrx.com/internal/stores/filechat"
)

func TestHubSubscribeReplaysAndBroadcasts(t *testing.T) {
	store, err := filechat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{T: 1, Type: "user", Text: "hi"}); err != nil {
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

func TestHubSubscribeAfterOnlyReplaysMissingEvents(t *testing.T) {
	store, err := filechat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), servicechat.Meta{ID: "abcd"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{T: 1, Type: "user", Text: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendEvent(context.Background(), "abcd", servicechat.Event{T: 2, Type: "assistant_text", Text: "second"}); err != nil {
		t.Fatal(err)
	}

	hub := New(store)
	sub, err := hub.SubscribeAfter(context.Background(), "abcd", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if ev := receiveEvent(t, sub); ev.Type != "assistant_text" || ev.Seq != 2 {
		t.Fatalf("unexpected replay event: %#v", ev)
	}
	if ev := receiveEvent(t, sub); ev.Type != "sync" || ev.Running {
		t.Fatalf("unexpected sync event: %#v", ev)
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

	if _, ok := hub.StartRun("abcd", func() {}); ok {
		t.Fatal("new run started before canceled owner quiesced")
	}
	hub.FinishRun("abcd", runID)
	secondRunID, ok := hub.StartRun("abcd", func() {})
	if !ok {
		t.Fatal("new run should start after canceled owner finishes")
	}
	hub.FinishRun("abcd", secondRunID)
}

func TestHubPublishesRunningTransitions(t *testing.T) {
	hub := New(nil)
	updates := make(chan bool, 2)
	hub.SetRunningSubscriber(func(id servicechat.ID, running bool) {
		if id == "abcd" {
			updates <- running
		}
	})

	runID, ok := hub.StartRun("abcd", func() {})
	if !ok {
		t.Fatal("run should start")
	}
	if running := receiveRunning(t, updates); !running {
		t.Fatal("expected running=true update")
	}

	hub.FinishRun("abcd", runID)
	if running := receiveRunning(t, updates); running {
		t.Fatal("expected running=false update")
	}
}

func TestHubEmitAllowsAppendNotificationsToReadRunning(t *testing.T) {
	running := make(chan bool, 1)
	var hub *Hub
	store := callbackStore{
		append: func() {
			running <- hub.IsRunning("abcd")
		},
	}
	hub = New(store)

	runID, ok := hub.StartRun("abcd", func() {})
	if !ok {
		t.Fatal("run should start")
	}

	done := make(chan struct{})
	go func() {
		hub.Emit("abcd", servicechat.Event{T: 1, Type: "user", Text: "hi"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("emit deadlocked while append notification read running state")
	}
	if isRunning := receiveRunning(t, running); !isRunning {
		t.Fatal("expected append notification to observe running chat")
	}

	hub.FinishRun("abcd", runID)
}

func TestStartRunWithSnapshotsAfterIdleChangeCommits(t *testing.T) {
	hub := New(nil)
	changeStarted := make(chan struct{})
	releaseChange := make(chan struct{})
	changeDone := make(chan struct{})
	state := "before"
	go func() {
		idle, err := hub.WhileIdle("abcd", func() error {
			close(changeStarted)
			<-releaseChange
			state = "after"
			return nil
		})
		if !idle || err != nil {
			panic("idle change failed")
		}
		close(changeDone)
	}()
	<-changeStarted

	type startResult struct {
		id       uint64
		started  bool
		err      error
		snapshot string
	}
	startedRun := make(chan startResult, 1)
	go func() {
		result := startResult{}
		result.id, result.started, result.err = hub.StartRunWith("abcd", func() {}, func() error {
			result.snapshot = state
			return nil
		})
		startedRun <- result
	}()
	select {
	case result := <-startedRun:
		t.Fatalf("run crossed an uncommitted idle change: %#v", result)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseChange)
	<-changeDone
	result := <-startedRun
	if result.err != nil || !result.started || result.snapshot != "after" {
		t.Fatalf("start result = %#v", result)
	}
	hub.FinishRun("abcd", result.id)
}

func TestIdleChangeWaitsForRunSnapshotThenRejects(t *testing.T) {
	hub := New(nil)
	snapshotStarted := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	startedRun := make(chan uint64, 1)
	go func() {
		runID, started, err := hub.StartRunWith("abcd", func() {}, func() error {
			close(snapshotStarted)
			<-releaseSnapshot
			return nil
		})
		if err != nil || !started {
			panic("run did not start")
		}
		startedRun <- runID
	}()
	<-snapshotStarted

	changeCalled := make(chan struct{}, 1)
	changeResult := make(chan bool, 1)
	go func() {
		idle, err := hub.WhileIdle("abcd", func() error {
			changeCalled <- struct{}{}
			return nil
		})
		if err != nil {
			panic(err)
		}
		changeResult <- idle
	}()
	select {
	case idle := <-changeResult:
		t.Fatalf("idle check crossed an incomplete run snapshot: %v", idle)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseSnapshot)
	runID := <-startedRun
	if idle := <-changeResult; idle {
		t.Fatal("active run allowed an idle-only change")
	}
	select {
	case <-changeCalled:
		t.Fatal("rejected idle-only callback was invoked")
	default:
	}
	hub.FinishRun("abcd", runID)
}

func TestStartRunWithSnapshotFailureReleasesReservation(t *testing.T) {
	hub := New(nil)
	want := errors.New("snapshot failed")
	_, started, err := hub.StartRunWith("abcd", func() {}, func() error { return want })
	if !started || !errors.Is(err, want) {
		t.Fatalf("start = %v, err = %v", started, err)
	}
	if hub.IsRunning("abcd") {
		t.Fatal("snapshot failure left the run reserved")
	}
}

func TestCancelWaitsForRunSnapshotBoundary(t *testing.T) {
	hub := New(nil)
	snapshotStarted := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	startDone := make(chan struct{})
	runIDs := make(chan uint64, 1)
	go func() {
		runID, _, _ := hub.StartRunWith("abcd", func() {}, func() error {
			close(snapshotStarted)
			<-releaseSnapshot
			return nil
		})
		runIDs <- runID
		close(startDone)
	}()
	<-snapshotStarted

	cancelDone := make(chan bool, 1)
	go func() { cancelDone <- hub.CancelRun("abcd") }()
	select {
	case <-cancelDone:
		t.Fatal("cancel crossed an incomplete run snapshot")
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseSnapshot)
	<-startDone
	runID := <-runIDs
	if cancelled := <-cancelDone; !cancelled {
		t.Fatal("reserved run was not cancelled")
	}
	if !hub.IsRunning("abcd") {
		t.Fatal("cancel released the reservation before the owner finished")
	}
	hub.FinishRun("abcd", runID)
	if hub.IsRunning("abcd") {
		t.Fatal("finished canceled run remained active")
	}
}

func TestCancelKeepsIdleTransitionsBlockedUntilRunFinishes(t *testing.T) {
	hub := New(nil)
	cancelled := make(chan struct{})
	cancelCalls := 0
	runID, started := hub.StartRun("abcd", func() {
		cancelCalls++
		close(cancelled)
	})
	if !started {
		t.Fatal("run should start")
	}
	if !hub.CancelRun("abcd") {
		t.Fatal("cancel should signal the active run")
	}
	<-cancelled
	if !hub.CancelRun("abcd") {
		t.Fatal("repeat cancel should still report an active run")
	}
	if cancelCalls != 1 {
		t.Fatalf("cancel callback calls = %d, want 1", cancelCalls)
	}

	called := false
	idle, err := hub.WhileIdle("abcd", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if idle || called {
		t.Fatal("canceled-but-not-finished run allowed an idle-only transition")
	}

	hub.FinishRun("abcd", runID)
	idle, err = hub.WhileIdle("abcd", func() error {
		called = true
		return nil
	})
	if err != nil || !idle || !called {
		t.Fatalf("finished run did not release idle transition: idle=%v called=%v err=%v", idle, called, err)
	}
}

func TestCancelAndWhileIdleWaitsForOwnerAndBlocksReplacementRun(t *testing.T) {
	hub := New(nil)
	cancelled := make(chan struct{})
	runID, started := hub.StartRun("abcd", func() { close(cancelled) })
	if !started {
		t.Fatal("run should start")
	}

	changeCalled := make(chan struct{})
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- hub.CancelAndWhileIdle(context.Background(), "abcd", func() error {
			close(changeCalled)
			return nil
		})
	}()
	<-cancelled
	select {
	case <-changeCalled:
		t.Fatal("destructive change overtook the canceled owner")
	case <-time.After(25 * time.Millisecond):
	}

	startDone := make(chan bool, 1)
	go func() {
		_, ok := hub.StartRun("abcd", func() {})
		startDone <- ok
	}()
	select {
	case ok := <-startDone:
		t.Fatalf("replacement crossed exclusive transition: started=%v", ok)
	case <-time.After(25 * time.Millisecond):
	}

	hub.FinishRun("abcd", runID)
	if err := <-deleteDone; err != nil {
		t.Fatal(err)
	}
	<-changeCalled
	if ok := <-startDone; !ok {
		t.Fatal("replacement should start after destructive transition completes")
	}
}

func TestCancelAndWhileIdleDoesNotChangeAfterRequestCancellation(t *testing.T) {
	hub := New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := hub.CancelAndWhileIdle(ctx, "abcd", func() error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if called {
		t.Fatal("destructive change ran after request cancellation")
	}
}

type callbackStore struct {
	append func()
}

func (s callbackStore) ReadEvents(ctx context.Context, chatID servicechat.ID) ([]servicechat.Event, error) {
	return nil, nil
}

func (s callbackStore) ReadEventsAfter(
	ctx context.Context,
	chatID servicechat.ID,
	afterSeq int64,
) ([]servicechat.Event, error) {
	return nil, nil
}

func (s callbackStore) AppendEvent(
	ctx context.Context,
	chatID servicechat.ID,
	ev servicechat.Event,
) (servicechat.Event, error) {
	if s.append != nil {
		s.append()
	}
	ev.Seq = 1
	return ev, nil
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

func receiveRunning(t *testing.T, updates <-chan bool) bool {
	t.Helper()
	select {
	case running := <-updates:
		return running
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for running update")
		return false
	}
}
