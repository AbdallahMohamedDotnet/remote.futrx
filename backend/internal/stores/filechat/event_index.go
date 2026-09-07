package filechat

import (
	"context"
	"sync"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

type indexedEventLocation struct {
	offset int64
	length int64
	seq    int64
}

// indexedTurnRange addresses a complete transcript turn inside events. The end
// is exclusive, matching Go slice bounds.
type indexedTurnRange struct {
	startEvent int
	endEvent   int
	startSeq   int64
}

// chatIndexState is mutated only while the Store's per-chat lock is held.
// Different chats can still build and read their indexes concurrently.
type chatIndexState struct {
	indexedBytes int64
	eventOrdinal int64
	lastSeq      int64
	fileMtimeNS  int64
	prefixHash   uint64
	tailComplete bool

	events []indexedEventLocation
	turns  []indexedTurnRange

	currentTurnID      string
	currentTurnHasUser bool
}

type cachedChatIndex struct {
	state    chatIndexState
	lastUsed uint64
}

const (
	defaultMaxCachedIndexChats     = 10_000
	defaultMaxCachedIndexLocations = 2_000_000
)

// chatEventIndex caches derived offsets and turn boundaries for chats touched
// during this process lifetime. The Store's per-chat lock serializes each
// state's mutation; this mutex only protects the map itself.
type chatEventIndex struct {
	mu           sync.Mutex
	states       map[servicechat.ID]cachedChatIndex
	clock        uint64
	locations    int
	maxChats     int
	maxLocations int
}

func newChatEventIndex() *chatEventIndex {
	return &chatEventIndex{
		states:       make(map[servicechat.ID]cachedChatIndex),
		maxChats:     defaultMaxCachedIndexChats,
		maxLocations: defaultMaxCachedIndexLocations,
	}
}

func (index *chatEventIndex) snapshot(id servicechat.ID) (chatIndexState, bool) {
	index.mu.Lock()
	defer index.mu.Unlock()
	entry, found := index.states[id]
	if !found {
		return chatIndexState{}, false
	}
	index.clock++
	entry.lastUsed = index.clock
	index.states[id] = entry
	return entry.state, true
}

func (index *chatEventIndex) publish(id servicechat.ID, state chatIndexState) {
	index.mu.Lock()
	defer index.mu.Unlock()
	if previous, found := index.states[id]; found {
		index.locations -= chatIndexLocations(previous.state)
	}
	index.clock++
	index.states[id] = cachedChatIndex{state: state, lastUsed: index.clock}
	index.locations += chatIndexLocations(state)
	index.evict(id)
}

func (index *chatEventIndex) forget(ctx context.Context, id servicechat.ID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	index.remove(id)
	return nil
}

func (index *chatEventIndex) invalidate(id servicechat.ID) {
	index.mu.Lock()
	defer index.mu.Unlock()
	index.remove(id)
}

func (index *chatEventIndex) remove(id servicechat.ID) {
	if entry, found := index.states[id]; found {
		index.locations -= chatIndexLocations(entry.state)
		delete(index.states, id)
	}
}

func (index *chatEventIndex) evict(current servicechat.ID) {
	for index.overBudget() && len(index.states) > 1 {
		var oldestID servicechat.ID
		var oldestUse uint64
		for id, entry := range index.states {
			if id == current || (oldestID != "" && entry.lastUsed >= oldestUse) {
				continue
			}
			oldestID = id
			oldestUse = entry.lastUsed
		}
		if oldestID == "" {
			return
		}
		index.remove(oldestID)
	}
}

func (index *chatEventIndex) overBudget() bool {
	tooManyChats := index.maxChats > 0 && len(index.states) > index.maxChats
	tooManyLocations := index.maxLocations > 0 && index.locations > index.maxLocations
	return tooManyChats || tooManyLocations
}

func chatIndexLocations(state chatIndexState) int {
	return cap(state.events) + cap(state.turns)
}
