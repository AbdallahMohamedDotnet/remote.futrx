package claude

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/chat"
)

type Hub struct {
	store *chat.ChatStore
	mu    sync.Mutex
	rooms map[string]*room
}

type room struct {
	mu      sync.Mutex
	subs    map[*Subscription]struct{}
	running *runState
	nextRun uint64
}

type runState struct {
	id     uint64
	cancel context.CancelFunc
}

const subscriptionLiveBuffer = 4096

type Subscription struct {
	room   *room
	events chan ChatEvent
	mu     sync.Mutex
	closed bool
}

func NewHub(store *chat.ChatStore) *Hub {
	return &Hub{
		store: store,
		rooms: map[string]*room{},
	}
}

func (h *Hub) room(chatID string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[chatID]; ok {
		return r
	}
	r := &room{subs: map[*Subscription]struct{}{}}
	h.rooms[chatID] = r
	return r
}

func (h *Hub) Subscribe(chatID string) (*Subscription, error) {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()

	replay, err := h.store.ReadEvents(chatID)
	if err != nil {
		return nil, err
	}

	sub := &Subscription{
		room:   r,
		events: make(chan ChatEvent, len(replay)+subscriptionLiveBuffer),
	}
	for _, ev := range replay {
		sub.events <- ev
	}
	sub.events <- ChatEvent{
		T:       time.Now().UnixMilli(),
		Type:    "sync",
		Running: r.running != nil,
	}
	r.subs[sub] = struct{}{}
	return sub, nil
}

func (s *Subscription) Events() <-chan ChatEvent {
	return s.events
}

func (s *Subscription) Close() {
	s.room.mu.Lock()
	delete(s.room.subs, s)
	s.room.mu.Unlock()
	s.closeChannel()
}

func (s *Subscription) SendTransient(ev ChatEvent) {
	_ = s.trySend(ev)
}

func (s *Subscription) trySend(ev ChatEvent) bool {
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

func (h *Hub) Emit(chatID string, ev ChatEvent) {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := h.store.AppendEvent(chatID, ev); err != nil {
		log.Printf("chat %s append: %v", chatID, err)
	}
	r.broadcastLocked(ev)
}

func (h *Hub) BroadcastTransient(chatID string, ev ChatEvent) {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcastLocked(ev)
}

func (r *room) broadcastLocked(ev ChatEvent) {
	for sub := range r.subs {
		if sub.trySend(ev) {
			continue
		}
		delete(r.subs, sub)
		sub.closeChannel()
	}
}

func (h *Hub) StartRun(chatID string, cancel context.CancelFunc) (uint64, bool) {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running != nil {
		return 0, false
	}
	r.nextRun++
	id := r.nextRun
	r.running = &runState{id: id, cancel: cancel}
	r.broadcastLocked(ChatEvent{
		T:       time.Now().UnixMilli(),
		Type:    "sync",
		Running: true,
	})
	return id, true
}

func (h *Hub) FinishRun(chatID string, runID uint64) {
	r := h.room(chatID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running != nil && r.running.id == runID {
		r.running = nil
		r.broadcastLocked(ChatEvent{
			T:    time.Now().UnixMilli(),
			Type: "sync",
		})
	}
}

func (h *Hub) CancelRun(chatID string) bool {
	r := h.room(chatID)
	r.mu.Lock()
	running := r.running
	r.mu.Unlock()
	if running == nil {
		return false
	}
	running.cancel()
	return true
}
