package presence

import "time"

// TTL is how long one heartbeat keeps a claim alive. It has to outlast the
// client's heartbeat interval by enough to ride out a dropped request, and
// stay short enough that a browser that dies without saying goodbye starts
// notifying its owner again quickly.
const TTL = 55 * time.Second

// maxClientsPerUser bounds one user's tracked clients. Claims expire anyway,
// so this only guards against a pathological reload loop minting ids faster
// than they age out.
const maxClientsPerUser = 20

// clientPresence is one client's report: the chat it says it has on screen,
// and when it last said so.
type clientPresence struct {
	chatID string
	seenAt time.Time
}

func (p clientPresence) isLive(now time.Time) bool {
	return p.seenAt.After(now.Add(-TTL))
}

// userPresence is one user's set of live claims, keyed by client. It owns the
// two rules that keep the set honest — a claim expires, and one user cannot
// hold more than the cap — so no caller has to remember either.
type userPresence struct {
	byClient map[string]clientPresence
}

func newUserPresence() *userPresence {
	return &userPresence{byClient: map[string]clientPresence{}}
}

// claim stores a client's presence, retiring expired ones first so that clients
// long gone never crowd out a live one.
func (p *userPresence) claim(clientID, chatID string, now time.Time) {
	p.retireExpired(now)
	if _, known := p.byClient[clientID]; !known && len(p.byClient) >= maxClientsPerUser {
		p.evictOldest()
	}
	p.byClient[clientID] = clientPresence{chatID: chatID, seenAt: now}
}

func (p *userPresence) release(clientID string) {
	delete(p.byClient, clientID)
}

func (p *userPresence) isEmpty() bool {
	return len(p.byClient) == 0
}

func (p *userPresence) isWatching(chatID string, now time.Time) bool {
	for _, client := range p.byClient {
		if client.chatID == chatID && client.isLive(now) {
			return true
		}
	}
	return false
}

func (p *userPresence) retireExpired(now time.Time) {
	for clientID, client := range p.byClient {
		if !client.isLive(now) {
			delete(p.byClient, clientID)
		}
	}
}

func (p *userPresence) evictOldest() {
	var oldestID string
	var oldest time.Time
	for clientID, client := range p.byClient {
		if oldestID == "" || client.seenAt.Before(oldest) {
			oldestID, oldest = clientID, client.seenAt
		}
	}
	delete(p.byClient, oldestID)
}
