package runhub

import (
	"context"
	"log"
	"sync"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

type EventStore interface {
	ReadEvents(ctx context.Context, chatID servicechat.ID) ([]servicechat.Event, error)
	ReadEventsAfter(ctx context.Context, chatID servicechat.ID, afterSeq int64) ([]servicechat.Event, error)
	AppendEvent(ctx context.Context, chatID servicechat.ID, ev servicechat.Event) (servicechat.Event, error)
}

type Hub struct {
	store             EventStore
	mu                sync.Mutex
	rooms             map[servicechat.ID]*room
	runningSubscriber func(servicechat.ID, bool)
}

type room struct {
	// transitionMu serializes metadata changes that require an idle chat with
	// run reservation plus its metadata snapshot. It is intentionally distinct
	// from mu: repository notifications may call IsRunning while a transition
	// is in progress.
	transitionMu sync.Mutex
	mu           sync.Mutex
	subs         map[*Subscription]struct{}
	running      *runState
	nextRun      uint64
}

type runState struct {
	id              uint64
	cancel          context.CancelFunc
	done            chan struct{}
	cancelRequested bool
}

const subscriptionLiveBuffer = 4096

type Subscription struct {
	room   *room
	events chan servicechat.Event
	mu     sync.Mutex
	closed bool
}

func New(store EventStore) *Hub {
	return &Hub{
		store: store,
		rooms: map[servicechat.ID]*room{},
	}
}

func (h *Hub) SetRunningSubscriber(fn func(servicechat.ID, bool)) {
	h.mu.Lock()
	h.runningSubscriber = fn
	h.mu.Unlock()
}

func (h *Hub) room(chatID servicechat.ID) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[chatID]; ok {
		return r
	}
	r := &room{subs: map[*Subscription]struct{}{}}
	h.rooms[chatID] = r
	return r
}

func (h *Hub) Subscribe(ctx context.Context, chatID servicechat.ID) (*Subscription, error) {
	return h.SubscribeAfter(ctx, chatID, 0)
}

func (h *Hub) SubscribeAfter(
	ctx context.Context,
	chatID servicechat.ID,
	afterSeq int64,
) (*Subscription, error) {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()

	var replay []servicechat.Event
	if h.store != nil {
		var events []servicechat.Event
		var err error
		if afterSeq > 0 {
			events, err = h.store.ReadEventsAfter(ctx, chatID, afterSeq)
		} else {
			events, err = h.store.ReadEvents(ctx, chatID)
		}
		if err != nil {
			return nil, err
		}
		replay = events
	}

	sub := &Subscription{
		room:   r,
		events: make(chan servicechat.Event, len(replay)+subscriptionLiveBuffer),
	}
	for _, ev := range replay {
		sub.events <- ev
	}
	sub.events <- servicechat.Event{
		T:       time.Now().UnixMilli(),
		Type:    "sync",
		Running: r.running != nil,
	}
	r.subs[sub] = struct{}{}
	return sub, nil
}

func (s *Subscription) Events() <-chan servicechat.Event {
	return s.events
}

func (s *Subscription) Close() {
	s.room.mu.Lock()
	delete(s.room.subs, s)
	s.room.mu.Unlock()
	s.closeChannel()
}

func (s *Subscription) SendTransient(ev servicechat.Event) {
	_ = s.trySend(ev)
}

func (s *Subscription) trySend(ev servicechat.Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.events <- ev:
		return true
	default:
		return false
	}
}

func (s *Subscription) closeChannel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.events)
}

func (h *Hub) Emit(chatID servicechat.ID, ev servicechat.Event) {
	stored := ev
	if h.store != nil {
		next, err := h.store.AppendEvent(context.Background(), chatID, ev)
		if err != nil {
			log.Printf("chat %s append: %v", chatID, err)
		} else {
			stored = next
		}
	}

	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcastLocked(stored)
}

func (h *Hub) BroadcastTransient(chatID servicechat.ID, ev servicechat.Event) {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcastLocked(ev)
}

func (r *room) broadcastLocked(ev servicechat.Event) {
	for sub := range r.subs {
		if sub.trySend(ev) {
			continue
		}
		delete(r.subs, sub)
		sub.closeChannel()
	}
}

func (h *Hub) StartRun(chatID servicechat.ID, cancel context.CancelFunc) (uint64, bool) {
	runID, started, _ := h.StartRunWith(chatID, cancel, nil)
	return runID, started
}

// StartRunWith reserves the run and executes snapshot before another idle-only
// metadata transition may commit. A snapshot failure releases the reservation
// before returning; started still reports whether this call won the run race.
func (h *Hub) StartRunWith(
	chatID servicechat.ID,
	cancel context.CancelFunc,
	snapshot func() error,
) (uint64, bool, error) {
	r := h.room(chatID)
	r.transitionMu.Lock()
	defer r.transitionMu.Unlock()

	r.mu.Lock()
	if r.running != nil {
		r.mu.Unlock()
		return 0, false, nil
	}
	r.nextRun++
	id := r.nextRun
	r.running = &runState{id: id, cancel: cancel, done: make(chan struct{})}
	r.broadcastLocked(servicechat.Event{
		T:       time.Now().UnixMilli(),
		Type:    "sync",
		Running: true,
	})
	r.mu.Unlock()
	h.publishRunning(chatID, true)

	if snapshot != nil {
		if err := snapshot(); err != nil {
			h.FinishRun(chatID, id)
			return 0, true, err
		}
	}
	return id, true, nil
}

// WhileIdle executes change only when no run is active and prevents a new run
// from reserving the chat until change has fully committed.
func (h *Hub) WhileIdle(chatID servicechat.ID, change func() error) (bool, error) {
	r := h.room(chatID)
	r.transitionMu.Lock()
	defer r.transitionMu.Unlock()

	r.mu.Lock()
	idle := r.running == nil
	r.mu.Unlock()
	if !idle {
		return false, nil
	}
	if change == nil {
		return true, nil
	}
	return true, change()
}

func (h *Hub) FinishRun(chatID servicechat.ID, runID uint64) {
	r := h.room(chatID)
	r.mu.Lock()
	changed := false
	var done chan struct{}
	if r.running != nil && r.running.id == runID {
		done = r.running.done
		r.running = nil
		r.broadcastLocked(servicechat.Event{
			T:    time.Now().UnixMilli(),
			Type: "sync",
		})
		changed = true
	}
	r.mu.Unlock()
	if changed {
		h.publishRunning(chatID, false)
		close(done)
	}
}

func (h *Hub) IsRunning(chatID servicechat.ID) bool {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running != nil
}

// CancelAndWhileIdle owns the full destructive transition: it prevents new
// runs and metadata changes, requests cancellation of an active owner, waits
// for that owner to finish all output and teardown, then executes change.
func (h *Hub) CancelAndWhileIdle(
	ctx context.Context,
	chatID servicechat.ID,
	change func() error,
) error {
	r := h.room(chatID)
	r.transitionMu.Lock()
	defer r.transitionMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	running := r.running
	if running != nil {
		firstRequest := !running.cancelRequested
		running.cancelRequested = true
		done := running.done
		cancel := running.cancel
		r.mu.Unlock()
		if firstRequest {
			cancel()
		}
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	} else {
		r.mu.Unlock()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if change == nil {
		return nil
	}
	return change()
}

func (h *Hub) CancelRun(chatID servicechat.ID) bool {
	r := h.room(chatID)
	r.transitionMu.Lock()
	defer r.transitionMu.Unlock()

	r.mu.Lock()
	running := r.running
	if running == nil {
		r.mu.Unlock()
		return false
	}
	firstRequest := !running.cancelRequested
	running.cancelRequested = true
	r.mu.Unlock()

	// Cancellation is only a signal. Keep the reservation until the owner
	// goroutine calls FinishRun after the provider and its teardown have fully
	// quiesced; otherwise a rewind or replacement run can overtake late output.
	if firstRequest {
		running.cancel()
	}
	return true
}

func (h *Hub) publishRunning(chatID servicechat.ID, running bool) {
	h.mu.Lock()
	fn := h.runningSubscriber
	h.mu.Unlock()
	if fn != nil {
		fn(chatID, running)
	}
}
