package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/agent/runtime"
)

const (
	appServerInitializeRequestID = 1
	appServerThreadRequestID     = 2
	appServerTurnRequestID       = 3
)

type appServerEnvelope struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type appServerThreadResult struct {
	Thread struct {
		ID string `json:"id"`
	} `json:"thread"`
	Model string `json:"model"`
}

// runAppServer owns one Codex app-server process for one Remote turn. A fresh
// transport can still resume or fork a persisted Codex thread, so no daemon is
// required between user messages.
func runAppServer(
	ctx context.Context,
	cmd *exec.Cmd,
	req agent.RunRequest,
	emit func(agent.Event),
) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn codex app-server: %w", err)
	}

	stderrDone := make(chan string, 1)
	go captureAppServerStderr(stderr, req.ConversationID, stderrDone)

	encoder := json.NewEncoder(stdin)
	writeRPC := func(message any) error { return encoder.Encode(message) }
	if err := writeRPC(map[string]any{
		"method": "initialize",
		"id":     appServerInitializeRequestID,
		"params": map[string]any{
			"clientInfo": map[string]string{
				"name":    "remote-futrx",
				"title":   "Remote",
				"version": "1",
			},
			"capabilities": map[string]bool{"experimentalApi": true},
		},
	}); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		<-stderrDone
		return err
	}

	parser := newAppServerEventParser(req)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	terminal := false
	runFailed := false
	var protocolErr error

	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(line) == 0 {
			continue
		}
		var envelope appServerEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			log.Printf("codex[%s] app-server parse: %v", req.ConversationID, err)
			continue
		}

		if envelope.Method != "" && len(envelope.ID) > 0 {
			if err := answerAppServerRequest(writeRPC, envelope, req, emit); err != nil {
				protocolErr = err
				break
			}
			continue
		}

		if envelope.Method != "" {
			for _, event := range parser.ParseNotification(envelope.Method, envelope.Params) {
				emit(event)
				if event.Type == agent.EventRunCompleted || event.Type == agent.EventRunFailed {
					terminal = true
					runFailed = event.Type == agent.EventRunFailed
					_ = stdin.Close()
				}
			}
			continue
		}

		responseID, ok := rpcResponseID(envelope.ID)
		if !ok {
			continue
		}
		if envelope.Error != nil {
			message := strings.TrimSpace(envelope.Error.Message)
			if responseID == appServerThreadRequestID && req.ResumeID != "" && isMissingCodexThread(message) {
				protocolErr = fmt.Errorf("%w: %s", agent.ErrSessionNotFound, message)
			} else {
				protocolErr = fmt.Errorf("codex app-server request %d: %s", responseID, message)
			}
			break
		}

		switch responseID {
		case appServerInitializeRequestID:
			if err := writeRPC(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
				protocolErr = err
				break
			}
			method, params := appServerThreadRequest(req)
			if err := writeRPC(map[string]any{
				"method": method,
				"id":     appServerThreadRequestID,
				"params": params,
			}); err != nil {
				protocolErr = err
			}

		case appServerThreadRequestID:
			var result appServerThreadResult
			if err := json.Unmarshal(envelope.Result, &result); err != nil {
				protocolErr = fmt.Errorf("decode Codex thread response: %w", err)
				break
			}
			if result.Thread.ID == "" || result.Model == "" {
				protocolErr = errors.New("Codex app-server returned an incomplete thread")
				break
			}
			if result.Thread.ID != req.ResumeID {
				emit(agent.Event{
					T:              time.Now().UnixMilli(),
					Type:           agent.EventSessionUpdated,
					Provider:       agent.ProviderCodex,
					ConversationID: req.ConversationID,
					SessionID:      result.Thread.ID,
					Model:          result.Model,
				})
			}
			if err := writeRPC(map[string]any{
				"method": "turn/start",
				"id":     appServerTurnRequestID,
				"params": appServerTurnParams(req, result.Thread.ID, result.Model),
			}); err != nil {
				protocolErr = err
			}

		case appServerTurnRequestID:
			// The turn response is only an acknowledgement. Streaming notifications
			// carry all user-visible state and the terminal status.
		}
		if protocolErr != nil {
			break
		}
	}

	if scanErr := scanner.Err(); scanErr != nil && ctx.Err() == nil && protocolErr == nil {
		protocolErr = fmt.Errorf("Codex app-server stdout: %w", scanErr)
	}
	if protocolErr != nil || !terminal {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	waitErr := cmd.Wait()
	stderrText := <-stderrDone

	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	if protocolErr != nil {
		return &agentruntime.ProcessError{Err: protocolErr, Stderr: stderrText}
	}
	if runFailed {
		return &agentruntime.ProcessError{Err: agent.ErrRunFailed, Stderr: stderrText}
	}
	if !terminal {
		if waitErr == nil {
			waitErr = errors.New("Codex app-server closed before the turn completed")
		}
		return &agentruntime.ProcessError{Err: waitErr, Stderr: stderrText}
	}
	return nil
}

func appServerThreadRequest(req agent.RunRequest) (string, map[string]any) {
	params := map[string]any{
		"approvalPolicy": "never",
		"sandbox":        "danger-full-access",
	}
	if cwd := strings.TrimSpace(req.Cwd); cwd != "" {
		params["cwd"] = cwd
	}
	if model := sanitizeModel(req.Model); model != "" {
		params["model"] = model
	}
	if tier := serviceTierArg(req.Preferences.ServiceTier); tier != "" {
		params["serviceTier"] = tier
	}
	if req.ResumeID == "" {
		params["serviceName"] = "remote-futrx"
		return "thread/start", params
	}
	params["threadId"] = req.ResumeID
	if req.Fork {
		return "thread/fork", params
	}
	return "thread/resume", params
}

func appServerTurnParams(req agent.RunRequest, threadID, model string) map[string]any {
	mode := "default"
	if req.Mode == "plan" {
		mode = "plan"
	}
	effort := reasoningEffortArg(req.Preferences.ReasoningEffort)
	if effort == "" && mode == "plan" {
		// Codex's native Plan preset uses medium reasoning when the user has not
		// selected an explicit effort.
		effort = "medium"
	}
	settings := map[string]any{
		"model":                  model,
		"developer_instructions": nil,
		"reasoning_effort":       nullableString(effort),
	}
	params := map[string]any{
		"threadId":       threadID,
		"input":          []map[string]string{{"type": "text", "text": req.Prompt}},
		"approvalPolicy": "never",
		"sandboxPolicy":  map[string]string{"type": "dangerFullAccess"},
		"model":          model,
		"collaborationMode": map[string]any{
			"mode":     mode,
			"settings": settings,
		},
	}
	if effort != "" {
		params["effort"] = effort
	}
	if tier := serviceTierArg(req.Preferences.ServiceTier); tier != "" {
		params["serviceTier"] = tier
	}
	return params
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func rpcResponseID(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var id int
	if err := json.Unmarshal(raw, &id); err != nil {
		return 0, false
	}
	return id, true
}

func isMissingCodexThread(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "not found") || strings.Contains(lower, "no rollout")
}

func captureAppServerStderr(reader io.Reader, logID string, done chan<- string) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 8192), 1<<20)
	var captured strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("codex[%s] stderr: %s", logID, line)
		if captured.Len() < 64<<10 {
			captured.WriteString(line)
			captured.WriteByte('\n')
		}
	}
	done <- captured.String()
}

func answerAppServerRequest(
	writeRPC func(any) error,
	envelope appServerEnvelope,
	req agent.RunRequest,
	emit func(agent.Event),
) error {
	result := any(nil)
	switch envelope.Method {
	case "item/tool/requestUserInput", "tool/requestUserInput":
		var params struct {
			ItemID    string `json:"itemId"`
			Questions []struct {
				Header   string `json:"header"`
				ID       string `json:"id"`
				Question string `json:"question"`
				Options  []struct {
					Label       string `json:"label"`
					Description string `json:"description"`
				} `json:"options"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(envelope.Params, &params); err != nil {
			return err
		}
		input, _ := json.Marshal(map[string]any{"questions": params.Questions})
		emit(agent.Event{
			T:              time.Now().UnixMilli(),
			Type:           agent.EventToolStarted,
			Provider:       agent.ProviderCodex,
			ConversationID: req.ConversationID,
			ItemID:         params.ItemID,
			ItemKind:       agent.ItemToolCall,
			ToolName:       "AskUserQuestion",
			Input:          input,
		})
		answers := make(map[string]any, len(params.Questions))
		for _, question := range params.Questions {
			answers[question.ID] = map[string]any{"answers": []string{}}
		}
		result = map[string]any{"answers": answers}

	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		decision := "accept"
		if req.Mode == "plan" {
			decision = "decline"
		}
		result = map[string]string{"decision": decision}

	case "execCommandApproval", "applyPatchApproval":
		if req.Mode == "plan" {
			result = map[string]any{"decision": map[string]any{
				"denied": map[string]string{"rejection": "Plan mode does not allow mutations"},
			}}
		} else {
			result = map[string]string{"decision": "approved"}
		}

	case "mcpServer/elicitation/request":
		result = map[string]any{"action": "cancel", "content": nil}

	default:
		return writeRPC(map[string]any{
			"id": envelope.ID,
			"error": map[string]any{
				"code":    -32601,
				"message": "Remote does not implement " + envelope.Method,
			},
		})
	}
	return writeRPC(map[string]any{"id": envelope.ID, "result": result})
}
