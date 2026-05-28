package main

import (
	"bufio"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChatMeta lives at data/chats/{id}/meta.json and is updated whenever the
// claude session id or last-message timestamp changes.
type ChatMeta struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	ClaudeSessionID string `json:"claudeSessionId,omitempty"`
	TmuxSession     string `json:"tmuxSession,omitempty"`
	Cwd             string `json:"cwd,omitempty"`
	CreatedAt       int64  `json:"createdAt"`
	LastMessageAt   int64  `json:"lastMessageAt"`
	Model           string `json:"model,omitempty"`
}

// ChatEvent is the normalized shape we persist and stream. It mirrors the
// TypeScript ChatEvent union in src/types.ts.
type ChatEvent struct {
	T               int64           `json:"t"`
	Type            string          `json:"type"`
	Text            string          `json:"text,omitempty"`
	MessageID       string          `json:"messageId,omitempty"`
	ID              string          `json:"id,omitempty"`
	Name            string          `json:"name,omitempty"`
	Input           json.RawMessage `json:"input,omitempty"`
	Output          string          `json:"output,omitempty"`
	IsError         bool            `json:"isError,omitempty"`
	ToolName        string          `json:"toolName,omitempty"`
	Subtype         string          `json:"subtype,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
	ClaudeSessionID string          `json:"claudeSessionId,omitempty"`
	Usage           json.RawMessage `json:"usage,omitempty"`
	Message         string          `json:"message,omitempty"`
}

// ChatStore manages chat dirs on disk. Single writer per chat via a per-id
// mutex map; concurrent access across different chats is fine.
type ChatStore struct {
	root  string
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewChatStore(root string) (*ChatStore, error) {
	if err := os.MkdirAll(filepath.Join(root, "chats"), 0o755); err != nil {
		return nil, err
	}
	return &ChatStore{root: root, locks: map[string]*sync.Mutex{}}, nil
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

func (s *ChatStore) GetMeta(id string) (ChatMeta, error) {
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
	return meta, nil
}

func (s *ChatStore) List() ([]ChatMeta, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "chats"))
	if err != nil {
		if os.IsNotExist(err) {
			return []ChatMeta{}, nil
		}
		return nil, err
	}
	out := make([]ChatMeta, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		meta, err := s.GetMeta(e.Name())
		if err != nil {
			continue
		}
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastMessageAt > out[j].LastMessageAt })
	return out, nil
}

func (s *ChatStore) Delete(id string) error {
	if !validChatID(id) {
		return errors.New("invalid chat id")
	}
	return os.RemoveAll(s.chatDir(id))
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
			_ = s.writeMeta(meta)
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

// --- HTTP -------------------------------------------------------------------

func (s *ChatStore) handleChatsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		metas, err := s.List()
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		sendJSON(w, 200, metas)
	case http.MethodPost:
		var body struct {
			Title       string `json:"title,omitempty"`
			TmuxSession string `json:"tmuxSession,omitempty"`
			Cwd         string `json:"cwd,omitempty"`
			Model       string `json:"model,omitempty"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil && err != io.EOF {
			sendErr(w, 400, "invalid json")
			return
		}
		// Resolve cwd from tmux if a session is given.
		cwd := body.Cwd
		if cwd == "" && body.TmuxSession != "" {
			if !validName.MatchString(body.TmuxSession) {
				sendErr(w, 400, "invalid tmuxSession")
				return
			}
			c, err := tmuxCwd(body.TmuxSession)
			if err == nil {
				cwd = c
			}
		}
		meta, err := s.Create(ChatMeta{
			Title:       body.Title,
			TmuxSession: body.TmuxSession,
			Cwd:         cwd,
			Model:       body.Model,
		})
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		sendJSON(w, 201, meta)
	default:
		sendErr(w, 405, "method not allowed")
	}
}

func (s *ChatStore) handleChatResource(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	if !validChatID(id) {
		sendErr(w, 400, "invalid chat id")
		return
	}

	// /api/chats/{id}/events
	if len(parts) == 2 && parts[1] == "events" {
		if r.Method != http.MethodGet {
			sendErr(w, 405, "method not allowed")
			return
		}
		events, err := s.ReadEvents(id)
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		sendJSON(w, 200, events)
		return
	}

	// /api/chats/{id}/upload — multipart upload into chat's cwd
	if len(parts) == 2 && parts[1] == "upload" {
		if r.Method != http.MethodPost {
			sendErr(w, 405, "method not allowed")
			return
		}
		meta, err := s.GetMeta(id)
		if err != nil {
			sendErr(w, 404, "chat not found")
			return
		}
		cwd := meta.Cwd
		if meta.TmuxSession != "" {
			if c, e := tmuxCwd(meta.TmuxSession); e == nil && c != "" {
				cwd = c
			}
		}
		if cwd == "" {
			cwd = os.Getenv("HOME")
			if cwd == "" {
				cwd = "/root"
			}
		}
		handleMultipartUpload(cwd, w, r)
		return
	}

	// /api/chats/{id}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			meta, err := s.GetMeta(id)
			if err != nil {
				if os.IsNotExist(err) {
					sendErr(w, 404, "not found")
				} else {
					sendErr(w, 500, err.Error())
				}
				return
			}
			sendJSON(w, 200, meta)
		case http.MethodPatch:
			var body struct {
				Title *string `json:"title,omitempty"`
				Cwd   *string `json:"cwd,omitempty"`
				Model *string `json:"model,omitempty"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
				sendErr(w, 400, "invalid json")
				return
			}
			meta, err := s.UpdateMeta(id, func(m *ChatMeta) {
				if body.Title != nil {
					m.Title = strings.TrimSpace(*body.Title)
				}
				if body.Cwd != nil {
					m.Cwd = *body.Cwd
				}
				if body.Model != nil {
					m.Model = *body.Model
				}
			})
			if err != nil {
				sendErr(w, 500, err.Error())
				return
			}
			sendJSON(w, 200, meta)
		case http.MethodDelete:
			if err := s.Delete(id); err != nil {
				sendErr(w, 500, err.Error())
				return
			}
			sendJSON(w, 200, map[string]bool{"ok": true})
		default:
			sendErr(w, 405, "method not allowed")
		}
		return
	}

	sendErr(w, 404, "not found")
}

// titleFromPrompt produces a short summary used when a chat is created with
// no explicit title. First 60 chars of the first prompt, single line.
func titleFromPrompt(prompt string) string {
	t := strings.TrimSpace(prompt)
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ReplaceAll(t, "\r", " ")
	for strings.Contains(t, "  ") {
		t = strings.ReplaceAll(t, "  ", " ")
	}
	if len(t) > 60 {
		t = t[:60] + "…"
	}
	if t == "" {
		t = fmt.Sprintf("Chat %s", time.Now().Format("Jan 2 15:04"))
	}
	return t
}
