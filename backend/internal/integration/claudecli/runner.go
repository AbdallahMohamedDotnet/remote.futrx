package claudecli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	servicechat "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/chat"
)

type ChatEvent = servicechat.Event

type Runner struct{}

func New() *Runner {
	return &Runner{}
}

// streamMsg is the on-wire JSON shape from `claude -p --output-format
// stream-json --include-partial-messages --verbose`. We parse the relevant
// subset; unknown fields are allowed.
type streamMsg struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Model     string          `json:"model,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Result    string          `json:"result,omitempty"`
	Usage     json.RawMessage `json:"usage,omitempty"`
}

type streamInner struct {
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

type message struct {
	ID      string         `json:"id,omitempty"`
	Role    string         `json:"role,omitempty"`
	Content []contentBlock `json:"content,omitempty"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

func (r *Runner) Run(
	ctx context.Context,
	id servicechat.ID,
	cmd *exec.Cmd,
	currentSessionID string,
	emit func(ChatEvent),
	updateSession func(sessionID, model string),
) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn claude: %w", err)
	}

	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 8192), 1<<20)
		for sc.Scan() {
			log.Printf("claude[%s] stderr: %s", id, sc.Text())
		}
	}()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	sawSessionID := currentSessionID
	var resultErr error

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw streamMsg
		if err := json.Unmarshal(line, &raw); err != nil {
			log.Printf("claude[%s] parse: %v line=%s", id, err, truncate(string(line), 200))
			continue
		}
		now := time.Now().UnixMilli()

		if raw.SessionID != "" && raw.SessionID != sawSessionID {
			sawSessionID = raw.SessionID
			if updateSession != nil {
				updateSession(raw.SessionID, raw.Model)
			}
			emit(ChatEvent{T: now, Type: "session", ClaudeSessionID: raw.SessionID})
		}

		switch raw.Type {
		case "system":
			emit(ChatEvent{T: now, Type: "system", Subtype: raw.Subtype, Data: raw.Message})

		case "stream_event":
			var inner streamInner
			if err := json.Unmarshal(raw.Event, &inner); err != nil {
				continue
			}
			if inner.Type == "content_block_delta" &&
				inner.Delta.Type == "text_delta" && inner.Delta.Text != "" {
				emit(ChatEvent{T: now, Type: "assistant_text", Text: inner.Delta.Text})
			}

		case "assistant":
			var m message
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
			var m message
			if err := json.Unmarshal(raw.Message, &m); err != nil {
				continue
			}
			for _, b := range m.Content {
				if b.Type == "tool_result" {
					emit(ChatEvent{
						T: now, Type: "tool_use_end",
						ID: b.ToolUseID, Output: normalizeToolResult(b.Content), IsError: false,
					})
				}
			}

		case "result":
			if raw.IsError {
				msg := strings.TrimSpace(raw.Result)
				if msg == "" {
					msg = "Claude returned an error"
				}
				emit(ChatEvent{T: now, Type: "error", Message: msg})
				resultErr = errors.New(msg)
				continue
			}
			emit(ChatEvent{T: now, Type: "complete", Usage: raw.Usage})
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		emit(ChatEvent{T: time.Now().UnixMilli(), Type: "error", Message: "stdout: " + err.Error()})
	}

	err = cmd.Wait()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	if resultErr != nil {
		return nil
	}
	return err
}
