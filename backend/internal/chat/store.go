package chat

import (
	"bufio"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ChatStore manages chat dirs on disk. Single writer per chat via a per-id
// mutex map; concurrent access across different chats is fine.
type ChatStore struct {
	root   string
	mu     sync.Mutex
	locks  map[string]*sync.Mutex
	metaMu sync.RWMutex
	metas  map[string]ChatMeta
}

func NewChatStore(root string) (*ChatStore, error) {
	if err := os.MkdirAll(filepath.Join(root, "chats"), 0o755); err != nil {
		return nil, err
	}
	store := &ChatStore{
		root:  root,
		locks: map[string]*sync.Mutex{},
		metas: map[string]ChatMeta{},
	}
	if err := store.loadMetaIndex(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *ChatStore) chatDir(id string) string {
	return filepath.Join(s.root, "chats", id)
}

func (s *ChatStore) lock(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.locks[id]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.locks[id] = m
	return m
}

func newChatID() string {
	var b [6]byte
	_, _ = crand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (s *ChatStore) Create(meta ChatMeta) (ChatMeta, error) {
	if meta.ID == "" {
		meta.ID = newChatID()
	}
	now := time.Now().UnixMilli()
	if meta.CreatedAt == 0 {
		meta.CreatedAt = now
	}
	if meta.LastMessageAt == 0 {
		meta.LastMessageAt = now
	}
	if meta.Title == "" {
		meta.Title = "New chat"
	}
	dir := s.chatDir(meta.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return meta, err
	}
	if err := s.writeMeta(meta); err != nil {
		return meta, err
	}
	s.setCachedMeta(meta)
	// Touch events file so reads don't 404.
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		f.Close()
	}
	return meta, nil
}

func (s *ChatStore) writeMeta(meta ChatMeta) error {
	dir := s.chatDir(meta.ID)
	tmp := filepath.Join(dir, "meta.json.tmp")
	final := filepath.Join(dir, "meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func (s *ChatStore) loadMetaIndex() error {
	entries, err := os.ReadDir(filepath.Join(s.root, "chats"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || !validChatID(e.Name()) {
			continue
		}
		meta, err := s.readMeta(e.Name())
		if err != nil {
			continue
		}
		s.metas[e.Name()] = meta
	}
	return nil
}

func (s *ChatStore) readMeta(id string) (ChatMeta, error) {
	if !validChatID(id) {
		return ChatMeta{}, errors.New("invalid chat id")
	}
	data, err := os.ReadFile(filepath.Join(s.chatDir(id), "meta.json"))
	if err != nil {
		return ChatMeta{}, err
	}
	var meta ChatMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return ChatMeta{}, err
	}
	if meta.ID == "" {
		meta.ID = id
	}
	return meta, nil
}

func (s *ChatStore) setCachedMeta(meta ChatMeta) {
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	s.metas[meta.ID] = meta
}

func (s *ChatStore) GetMeta(id string) (ChatMeta, error) {
	if !validChatID(id) {
		return ChatMeta{}, errors.New("invalid chat id")
	}
	s.metaMu.RLock()
	meta, ok := s.metas[id]
	s.metaMu.RUnlock()
	if ok {
		return meta, nil
	}
	meta, err := s.readMeta(id)
	if err != nil {
		return ChatMeta{}, err
	}
	s.setCachedMeta(meta)
	return meta, nil
}

// UpdateMeta applies a mutator under the chat's lock.
func (s *ChatStore) UpdateMeta(id string, fn func(*ChatMeta)) (ChatMeta, error) {
	if !validChatID(id) {
		return ChatMeta{}, errors.New("invalid chat id")
	}
	lk := s.lock(id)
	lk.Lock()
	defer lk.Unlock()
	meta, err := s.GetMeta(id)
	if err != nil {
		return ChatMeta{}, err
	}
	fn(&meta)
	if err := s.writeMeta(meta); err != nil {
		return meta, err
	}
	s.setCachedMeta(meta)
	return meta, nil
}

func (s *ChatStore) List() ([]ChatMeta, error) {
	s.metaMu.RLock()
	out := make([]ChatMeta, 0, len(s.metas))
	for _, meta := range s.metas {
		out = append(out, meta)
	}
	s.metaMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].LastMessageAt > out[j].LastMessageAt })
	return out, nil
}

func (s *ChatStore) Delete(id string) error {
	if !validChatID(id) {
		return errors.New("invalid chat id")
	}
	if err := os.RemoveAll(s.chatDir(id)); err != nil {
		return err
	}
	s.metaMu.Lock()
	delete(s.metas, id)
	s.metaMu.Unlock()
	s.mu.Lock()
	delete(s.locks, id)
	s.mu.Unlock()
	return nil
}

// AppendEvent writes one event to events.jsonl and bumps lastMessageAt.
// Safe for concurrent calls on the same chat (serialized via per-id lock).
func (s *ChatStore) AppendEvent(id string, ev ChatEvent) error {
	if !validChatID(id) {
		return errors.New("invalid chat id")
	}
	if ev.T == 0 {
		ev.T = time.Now().UnixMilli()
	}
	lk := s.lock(id)
	lk.Lock()
	defer lk.Unlock()

	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	f, err := os.OpenFile(
		filepath.Join(s.chatDir(id), "events.jsonl"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return err
	}
	// Bump lastMessageAt for user/assistant events; skip system noise.
	if ev.Type == "user" || ev.Type == "assistant_text" || ev.Type == "complete" {
		meta, err := s.GetMeta(id)
		if err == nil {
			meta.LastMessageAt = ev.T
			if err := s.writeMeta(meta); err == nil {
				s.setCachedMeta(meta)
			}
		}
	}
	return nil
}

func (s *ChatStore) ReadEvents(id string) ([]ChatEvent, error) {
	if !validChatID(id) {
		return nil, errors.New("invalid chat id")
	}
	f, err := os.Open(filepath.Join(s.chatDir(id), "events.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return []ChatEvent{}, nil
		}
		return nil, err
	}
	defer f.Close()
	out := make([]ChatEvent, 0, 64)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // up to 4MB per line
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev ChatEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip corrupt lines rather than abort
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}

func validChatID(id string) bool {
	if len(id) < 4 || len(id) > 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func ValidID(id string) bool {
	return validChatID(id)
}
