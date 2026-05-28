package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/chat"
	"github.com/gorilla/websocket"
)

type ChatStore = chat.ChatStore
type ChatEvent = chat.ChatEvent
type ChatMeta = chat.ChatMeta

type TmuxClient interface {
	Cwd(session string) (string, error)
}

type Runner struct {
	store *chat.ChatStore
	tmux  TmuxClient
	hub   *Hub
}

func NewRunner(store *chat.ChatStore, tmux TmuxClient) *Runner {
	return &Runner{store: store, tmux: tmux, hub: NewHub(store)}
}

// claudeStreamMsg is the on-wire JSON shape from `claude -p --output-format
// stream-json --include-partial-messages --verbose`. We parse the relevant
// subset; unknown fields are allowed (json package ignores them).
type claudeStreamMsg struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Model     string          `json:"model,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"` // for type=="stream_event"
	IsError   bool            `json:"is_error,omitempty"`
	Result    string          `json:"result,omitempty"`
	Usage     json.RawMessage `json:"usage,omitempty"`
}

// claudeStreamInner is the Anthropic-API streaming event wrapped by stream_event.
type claudeStreamInner struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type string `json:"type,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"delta,omitempty"`
	ContentBlock struct {
		Type  string          `json:"type,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content_block,omitempty"`
}

type claudeMessage struct {
	ID      string               `json:"id,omitempty"`
	Role    string               `json:"role,omitempty"`
	Content []claudeContentBlock `json:"content,omitempty"`
	// Usage and other fields are intentionally ignored here — we get usage
	// from the top-level `result` event instead.
}

type claudeContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"` // tool_result payload
}

// StreamHandler upgrades the WS, reads client prompts, spawns claude with
// stream-json, normalizes its events, persists them, and forwards to client.
func (rnr *Runner) StreamHandler(upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rnr.handleStream(upgrader, w, r)
	}
}

func (rnr *Runner) handleStream(upgrader websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/ws/chat/")
	if !chat.ValidID(id) {
		http.Error(w, "invalid chat id", http.StatusBadRequest)
		return
	}
	if _, err := rnr.store.GetMeta(id); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "chat not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sub, err := rnr.hub.Subscribe(id)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sub.Close()

	go func() {
		for ev := range sub.Events() {
			if err := conn.WriteJSON(ev); err != nil {
				_ = conn.Close()
				return
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Type     string `json:"type"`
			Text     string `json:"text,omitempty"`
			Approved bool   `json:"approved,omitempty"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "prompt":
			rnr.startPrompt(id, msg.Text, sub)

		case "cancel":
			if !rnr.hub.CancelRun(id) {
				sub.SendTransient(ChatEvent{
					T: time.Now().UnixMilli(), Type: "error",
					Message: "no prompt is currently running",
				})
			}
		}
	}
}

func (rnr *Runner) startPrompt(id, prompt string, requester *Subscription) {
	ctx, cancel := context.WithCancel(context.Background())
	runID, ok := rnr.hub.StartRun(id, cancel)
	if !ok {
		cancel()
		requester.SendTransient(ChatEvent{
			T: time.Now().UnixMilli(), Type: "error",
			Message: "a previous prompt is still running — cancel first",
		})
		return
	}

	go func() {
		defer rnr.hub.FinishRun(id, runID)
		rnr.runPrompt(
			ctx,
			id,
			prompt,
			func(ev ChatEvent) { rnr.hub.Emit(id, ev) },
			requester.SendTransient,
		)
	}()
}

func (rnr *Runner) runPrompt(
	ctx context.Context,
	id string,
	prompt string,
	emit func(ChatEvent),
	emitTransient func(ChatEvent),
) {
	meta, err := rnr.store.GetMeta(id)
	if err != nil {
		emitTransient(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: err.Error()})
		return
	}

	// Auto-title from first user prompt if still default.
	if meta.Title == "" || meta.Title == "New chat" {
		_, _ = rnr.store.UpdateMeta(id, func(m *ChatMeta) {
			m.Title = chat.TitleFromPrompt(prompt)
		})
	}

	// Resolve a fresh cwd: live tmux pane_current_path if linked, else stored.
	cwd := meta.Cwd
	if meta.TmuxSession != "" {
		if c, err := rnr.tmux.Cwd(meta.TmuxSession); err == nil && c != "" {
			cwd = c
		}
	}
	if cwd == "" {
		cwd = os.Getenv("HOME")
		if cwd == "" {
			cwd = "/root"
		}
	}

	// Persist the user message before spawning claude.
	emit(ChatEvent{T: time.Now().UnixMilli(), Type: "user", Text: prompt})

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
		"--dangerously-skip-permissions",
	}
	if meta.Model != "" {
		// Strip context-window suffixes like "[1m]" that may appear in older
		// stored metadata — claude --model doesn't accept those.
		modelArg := meta.Model
		if idx := strings.Index(modelArg, "["); idx > 0 {
			modelArg = modelArg[:idx]
		}
		args = append(args, "--model", modelArg)
	}
	if meta.ClaudeSessionID != "" {
		args = append(args, "--resume", meta.ClaudeSessionID)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = cwd
	// IS_SANDBOX=1 lets `claude --dangerously-skip-permissions` run under
	// uid 0 — claude otherwise refuses for safety. The box is single-user
	// and the user explicitly asked for auto-approve in this UI.
	cmd.Env = append(os.Environ(), "IS_SANDBOX=1")
	cmd.Stdin = strings.NewReader(prompt)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: err.Error()})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: err.Error()})
		return
	}
	if err := cmd.Start(); err != nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: "spawn claude: " + err.Error()})
		return
	}

	// Drain stderr to journal.
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 8192), 1<<20)
		for sc.Scan() {
			log.Printf("claude[%s] stderr: %s", id, sc.Text())
		}
	}()

	// Parse stdout line-by-line.
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20) // up to 16MB per line (tool outputs can be large)
	sawSessionID := meta.ClaudeSessionID

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw claudeStreamMsg
		if err := json.Unmarshal(line, &raw); err != nil {
			log.Printf("claude[%s] parse: %v line=%s", id, err, truncate(string(line), 200))
			continue
		}
		now := time.Now().UnixMilli()

		// Capture the session id on first sight + persist to meta.
		// Only set Model from the init event if the user hasn't picked one
		// explicitly — otherwise we'd overwrite an alias like "opus" with
		// the long-form name claude resolved internally.
		if raw.SessionID != "" && raw.SessionID != sawSessionID {
			sawSessionID = raw.SessionID
			_, _ = rnr.store.UpdateMeta(id, func(m *ChatMeta) {
				m.ClaudeSessionID = raw.SessionID
				if m.Model == "" && raw.Model != "" {
					m.Model = raw.Model
				}
			})
			emit(ChatEvent{T: now, Type: "session", ClaudeSessionID: raw.SessionID})
		}

		switch raw.Type {
		case "system":
			// init / model info — not very interesting to render, but keep a record.
			emit(ChatEvent{T: now, Type: "system", Subtype: raw.Subtype, Data: raw.Message})

		case "stream_event":
			// Token-level streaming. We emit text deltas immediately so the
			// UI can render words as they arrive. Tool use blocks we still
			// surface via the consolidated `assistant` event below — their
			// input is JSON streamed character-by-character which is useless
			// to render mid-flight.
			var inner claudeStreamInner
			if err := json.Unmarshal(raw.Event, &inner); err != nil {
				continue
			}
			if inner.Type == "content_block_delta" &&
				inner.Delta.Type == "text_delta" && inner.Delta.Text != "" {
				emit(ChatEvent{T: now, Type: "assistant_text", Text: inner.Delta.Text})
			}

		case "assistant":
			// With --include-partial-messages, text has already been streamed
			// via stream_event deltas — skip text blocks here to avoid dupes.
			// Tool use blocks aren't streamed (their JSON input would be
			// useless mid-flight) so we surface them here when complete.
			var m claudeMessage
			if err := json.Unmarshal(raw.Message, &m); err != nil {
				log.Printf("claude[%s] assistant parse: %v", id, err)
				continue
			}
			for _, b := range m.Content {
				switch b.Type {
				case "tool_use":
					emit(ChatEvent{
						T: now, Type: "tool_use_start",
						ID: b.ID, Name: b.Name, Input: b.Input,
					})
				case "thinking":
					if b.Text != "" {
						emit(ChatEvent{T: now, Type: "thinking", Text: b.Text})
					}
				}
			}

		case "user":
			// Tool results arrive packed inside a user-role message.
			var m claudeMessage
			if err := json.Unmarshal(raw.Message, &m); err != nil {
				continue
			}
			for _, b := range m.Content {
				if b.Type == "tool_result" {
					out := normalizeToolResult(b.Content)
					emit(ChatEvent{
						T: now, Type: "tool_use_end",
						ID: b.ToolUseID, Output: out, IsError: false,
					})
				}
			}

		case "result":
			emit(ChatEvent{T: now, Type: "complete", Usage: raw.Usage})
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: "stdout: " + err.Error()})
	}

	err = cmd.Wait()
	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	if err != nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: "claude exit: " + err.Error()})
	}
}
