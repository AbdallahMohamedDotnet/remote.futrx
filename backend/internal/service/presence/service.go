// Package presence tracks which chat each user is looking at right now.
//
// It exists because a service worker can only silence the device it runs on:
// with no shared signal, a phone still buzzes about a question its owner is
// reading on a laptop. Clients heartbeat what they are watching, and the push
// notifier skips a user whose own eyes are already on the chat.
//
// State is in-memory and expires on its own. It is a hint about this moment,
// nothing worth persisting, and the cost of losing it is one extra
// notification rather than a missed one.
package presence

import (
	"strings"
	"sync"
	"time"
)

// TTL is how long one heartbeat keeps a claim alive. It has to outlast the
// client's heartbeat interval by enough to ride out a dropped request, and
// stay short enough that a browser that dies without saying goodbye starts
// notifying its owner again quickly.
const TTL = 55 * time.Second

// maxClientsPerUser bounds one user's tracked clients. Claims expire anyway,
// so this only guards against a pathological reload loop minting ids faster
// than they age out.
const maxClientsPerUser = 20

// Service records which chat each of a user's clients is watching.
//
// Claims are tracked per client rather than per user so that clients stay
// independent: a background tab signing off cannot cancel the claim of the
// focused tab beside it, whichever order their requests land in.
type Service struct {
	clock func() time.Time

	mu    sync.Mutex
	users map[string]*clientClaims
}

func New() *Service {
	return newAt(time.Now)
}

// newAt builds a service reading a caller-supplied clock, so tests can age
// claims out without sleeping through the real TTL.
func newAt(clock func() time.Time) *Service {
	return &Service{clock: clock, users: map[string]*clientClaims{}}
}

// Claim records that one of a user's clients has a chat on screen. A blank
// user or chat cannot form a claim and is ignored.
func (s *Service) Claim(email, clientID, chatID string) {
	if s == nil {
		return
	}
	email, clientID, chatID = normalizeEmail(email), clientKey(clientID), trim(chatID)
	if email == "" || chatID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	claims := s.users[email]
	if claims == nil {
		claims = newClientClaims()
		s.users[email] = claims
	}
	claims.record(clientID, chatID, s.clock())
}

// Release withdraws one client's claim — it went to the background, lost
// focus, or navigated away — so the user hears from us again without waiting
// out the TTL.
func (s *Service) Release(email, clientID string) {
	if s == nil {
		return
	}
	email, clientID = normalizeEmail(email), clientKey(clientID)
	if email == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	claims := s.users[email]
	if claims == nil {
		return
	}
	claims.forget(clientID)
	if claims.empty() {
		delete(s.users, email)
	}
}

// IsWatching reports whether any of a user's live clients has this chat on
// screen.
func (s *Service) IsWatching(email, chatID string) bool {
	if s == nil {
		return false
	}
	email, chatID = normalizeEmail(email), trim(chatID)
	if email == "" || chatID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	claims := s.users[email]
	return claims != nil && claims.watching(chatID, s.clock())
}

// Filter drops the recipients who are already watching this chat. A nil
// service filters nothing, so a deployment without presence tracking notifies
// exactly as it did before.
func (s *Service) Filter(recipients []string, chatID string) []string {
	if s == nil || len(recipients) == 0 || trim(chatID) == "" {
		return recipients
	}
	kept := make([]string, 0, len(recipients))
	for _, email := range recipients {
		if s.IsWatching(email, chatID) {
			continue
		}
		kept = append(kept, email)
	}
	return kept
}

// claim is one client's report: the chat it says it has on screen, and when it
// last said so.
type claim struct {
	chatID string
	seenAt time.Time
}

func (c claim) live(now time.Time) bool {
	return c.seenAt.After(now.Add(-TTL))
}

// clientClaims is one user's set of live claims, keyed by client. It owns the
// two rules that keep the set honest — a claim expires, and one user cannot
// hold more than the cap — so no caller has to remember either.
type clientClaims struct {
	byClient map[string]claim
}

func newClientClaims() *clientClaims {
	return &clientClaims{byClient: map[string]claim{}}
}

// record stores a client's claim, retiring expired ones first so that clients
// long gone never crowd out a live one.
func (c *clientClaims) record(clientID, chatID string, now time.Time) {
	c.retire(now)
	if _, known := c.byClient[clientID]; !known && len(c.byClient) >= maxClientsPerUser {
		c.evictOldest()
	}
	c.byClient[clientID] = claim{chatID: chatID, seenAt: now}
}

func (c *clientClaims) forget(clientID string) {
	delete(c.byClient, clientID)
}

func (c *clientClaims) empty() bool {
	return len(c.byClient) == 0
}

func (c *clientClaims) watching(chatID string, now time.Time) bool {
	for _, held := range c.byClient {
		if held.chatID == chatID && held.live(now) {
			return true
		}
	}
	return false
}

func (c *clientClaims) retire(now time.Time) {
	for clientID, held := range c.byClient {
		if !held.live(now) {
			delete(c.byClient, clientID)
		}
	}
}

func (c *clientClaims) evictOldest() {
	var oldestID string
	var oldest time.Time
	for clientID, held := range c.byClient {
		if oldestID == "" || held.seenAt.Before(oldest) {
			oldestID, oldest = clientID, held.seenAt
		}
	}
	delete(c.byClient, oldestID)
}

// clientKey addresses one client. A caller that sends no id still gets a slot
// — as a single implicit client — rather than being silently untracked, and an
// oversized id is cut down so an opaque string cannot set the key size.
func clientKey(clientID string) string {
	clientID = trim(clientID)
	if clientID == "" {
		return "-"
	}
	if len(clientID) > 128 {
		return clientID[:128]
	}
	return clientID
}

// normalizeEmail matches the identity form the push subscription store keys
// on, so a presence claim and a subscription resolve to the same user.
func normalizeEmail(email string) string {
	return strings.ToLower(trim(email))
}

func trim(value string) string {
	return strings.TrimSpace(value)
}
