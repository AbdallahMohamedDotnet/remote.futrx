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
	"sync"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/integration/agents/runtime"
)

type appServerRun struct {
	ctx context.Context
	cmd *exec.Cmd
	req agent.RunRequest

	emit           func(agent.Event)
	stdin          io.WriteCloser
	scanner        *bufio.Scanner
	stderrDone     chan string
	write          func(any) error
	inputMu        sync.Mutex
	inputClosed    bool
	eventParser    *appServerEventParser
	requestHandler *appServerRequestHandler
	requestMu      sync.Mutex
	requests       map[string]*appServerPendingRequest
	requestWG      sync.WaitGroup
	requestErr     chan error
	terminal       bool
	runFailed      bool
	protocolErr    error

	threadID             string
	turnID               string
	pendingNotifications []appServerEnvelope
}

type appServerPendingRequest struct {
	cancel context.CancelFunc
	done   <-chan struct{}
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
	return newAppServerRun(ctx, cmd, req, emit).execute()
}

func newAppServerRun(
	ctx context.Context,
	cmd *exec.Cmd,
	req agent.RunRequest,
	emit func(agent.Event),
) *appServerRun {
	return &appServerRun{ctx: ctx, cmd: cmd, req: req, emit: emit}
}

func (run *appServerRun) execute() error {
	if err := run.start(); err != nil {
		return err
	}
	if err := run.initialize(); err != nil {
		run.abortInitialization()
		return err
	}
	run.consumeOutput()
	return run.finish()
}

func (run *appServerRun) start() error {
	stdin, err := run.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := run.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := run.cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := run.cmd.Start(); err != nil {
		return fmt.Errorf("spawn codex app-server: %w", err)
	}

	run.stdin = stdin
	run.stderrDone = make(chan string, 1)
	go captureAppServerStderr(stderr, run.req.ConversationID, run.stderrDone)
	encoder := json.NewEncoder(stdin)
	run.write = func(message any) error {
		run.inputMu.Lock()
		defer run.inputMu.Unlock()
		if run.inputClosed {
			return io.ErrClosedPipe
		}
		return encoder.Encode(message)
	}
	run.eventParser = newAppServerEventParser(run.req)
	run.requestHandler = newAppServerRequestHandler(run.req, run.write)
	run.requests = make(map[string]*appServerPendingRequest)
	run.requestErr = make(chan error, 1)
	run.scanner = bufio.NewScanner(stdout)
	run.scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	return nil
}

func (run *appServerRun) initialize() error {
	return run.write(map[string]any{
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
	})
}

func (run *appServerRun) abortInitialization() {
	_ = run.cmd.Process.Kill()
	_ = run.cmd.Wait()
	<-run.stderrDone
}

func (run *appServerRun) consumeOutput() {
	for run.scanner.Scan() {
		line := append([]byte(nil), run.scanner.Bytes()...)
		if len(line) == 0 {
			continue
		}
		var envelope appServerEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			log.Printf("codex[%s] app-server parse: %v", run.req.ConversationID, err)
			continue
		}
		if !run.handleEnvelope(envelope) {
			break
		}
	}
}

func (run *appServerRun) handleEnvelope(envelope appServerEnvelope) bool {
	if run.terminal {
		// The terminal event closed stdin and canceled every pending request.
		// Ignore any provider output already buffered behind that boundary rather
		// than exposing a card that can no longer receive a response.
		return true
	}
	if envelope.Method != "" && len(envelope.ID) > 0 {
		run.dispatchRequest(envelope)
		return true
	}
	if envelope.Method != "" {
		run.handleNotification(envelope)
		return true
	}

	responseID, ok := rpcResponseID(envelope.ID)
	if !ok {
		return true
	}
	if envelope.Error != nil {
		run.protocolErr = run.responseError(responseID, envelope.Error.Message)
		return false
	}
	run.handleResponse(responseID, envelope.Result)
	return run.protocolErr == nil
}

func (run *appServerRun) handleNotification(envelope appServerEnvelope) {
	if envelope.Method == "serverRequest/resolved" {
		// Requests are correlated across the whole connection, including those
		// from subagents. Keep resolving them independently of displayed events.
		run.cancelResolvedRequest(envelope.Params)
		return
	}
	var scope appServerNotificationScope
	if json.Unmarshal(envelope.Params, &scope) != nil || run.threadID == "" || scope.ThreadID != run.threadID {
		return
	}
	if run.turnID == "" {
		// Notifications can arrive before the turn/start response. Wait for its
		// authoritative ID rather than adopting a child or previously resumed turn.
		run.pendingNotifications = append(run.pendingNotifications, envelope)
		return
	}
	turnID := scope.TurnID
	if envelope.Method == "turn/started" || envelope.Method == "turn/completed" {
		turnID = scope.Turn.ID
	}
	if turnID != run.turnID {
		return
	}
	for _, event := range run.eventParser.ParseNotification(envelope.Method, envelope.Params) {
		if event.Type == agent.EventRunCompleted || event.Type == agent.EventRunFailed {
			// Resolve every parked card before publishing the terminal event. A
			// missing serverRequest/resolved notification must not reorder
			// cancellation after completion or hold process shutdown open.
			run.cancelAllRequests()
			run.requestWG.Wait()
			run.terminal = true
			run.runFailed = event.Type == agent.EventRunFailed
			_ = run.closeInput()
		}
		run.emit(event)
	}
}

func (run *appServerRun) dispatchRequest(envelope appServerEnvelope) {
	requestID := appServerRequestKey(envelope.ID)
	if requestID == "" {
		run.failRequest(errors.New("Codex app-server sent a request without an id"))
		return
	}
	requestCtx, cancel := context.WithCancel(run.ctx)
	workerDone := make(chan struct{})
	pending := &appServerPendingRequest{cancel: cancel, done: workerDone}

	run.requestMu.Lock()
	if _, exists := run.requests[requestID]; exists {
		run.requestMu.Unlock()
		cancel()
		run.failRequest(fmt.Errorf("Codex app-server reused pending request id %s", requestID))
		return
	}
	run.requests[requestID] = pending
	run.requestWG.Add(1)
	run.requestMu.Unlock()

	registered := make(chan struct{})
	var registeredOnce sync.Once
	markRegistered := func() {
		registeredOnce.Do(func() { close(registered) })
	}
	go func() {
		defer run.requestWG.Done()
		defer close(workerDone)
		err := run.requestHandler.answer(requestCtx, envelope, markRegistered)
		wasCancelled := requestCtx.Err() != nil

		run.requestMu.Lock()
		if run.requests[requestID] == pending {
			delete(run.requests, requestID)
		}
		run.requestMu.Unlock()
		cancel()

		if err != nil && !errors.Is(err, context.Canceled) && !wasCancelled {
			run.failRequest(err)
		}
	}()

	// Do not let a later delta/resolution overtake request registration. Once
	// the consumer has exposed the card, the worker may continue waiting while
	// the scanner resumes processing native output.
	select {
	case <-registered:
	case <-workerDone:
	case <-run.ctx.Done():
		cancel()
	}
}

func (run *appServerRun) cancelResolvedRequest(paramsJSON json.RawMessage) {
	var params struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(paramsJSON, &params) != nil {
		return
	}
	requestID := appServerRequestKey(params.RequestID)
	run.requestMu.Lock()
	pending := run.requests[requestID]
	run.requestMu.Unlock()
	if pending != nil {
		pending.cancel()
		<-pending.done
	}
}

func (run *appServerRun) cancelAllRequests() {
	run.requestMu.Lock()
	pending := make([]*appServerPendingRequest, 0, len(run.requests))
	for _, request := range run.requests {
		pending = append(pending, request)
	}
	run.requestMu.Unlock()
	for _, request := range pending {
		request.cancel()
	}
}

func (run *appServerRun) failRequest(err error) {
	if err == nil {
		return
	}
	select {
	case run.requestErr <- err:
	default:
	}
	if run.cmd.Process != nil {
		_ = run.cmd.Process.Kill()
	}
}

func (run *appServerRun) closeInput() error {
	run.inputMu.Lock()
	defer run.inputMu.Unlock()
	if run.inputClosed {
		return nil
	}
	run.inputClosed = true
	return run.stdin.Close()
}

func appServerRequestKey(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var textID string
	if json.Unmarshal(raw, &textID) == nil {
		textID = strings.TrimSpace(textID)
		if textID == "" {
			return ""
		}
		return "s:" + textID
	}
	var numericID int64
	if json.Unmarshal(raw, &numericID) == nil {
		return fmt.Sprintf("n:%d", numericID)
	}
	return ""
}

func (run *appServerRun) responseError(responseID int, message string) error {
	message = strings.TrimSpace(message)
	if responseID == appServerThreadRequestID && run.req.ResumeID != "" && isMissingCodexThread(message) {
		return fmt.Errorf("%w: %s", agent.ErrSessionNotFound, message)
	}
	return fmt.Errorf("codex app-server request %d: %s", responseID, message)
}

func (run *appServerRun) handleResponse(responseID int, resultJSON json.RawMessage) {
	switch responseID {
	case appServerInitializeRequestID:
		if err := run.write(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
			run.protocolErr = err
			return
		}
		request := buildAppServerThreadRequest(run.req)
		run.protocolErr = run.write(map[string]any{
			"method": request.Method,
			"id":     appServerThreadRequestID,
			"params": request.Params,
		})

	case appServerThreadRequestID:
		var result appServerThreadResult
		if err := json.Unmarshal(resultJSON, &result); err != nil {
			run.protocolErr = fmt.Errorf("decode Codex thread response: %w", err)
			return
		}
		if result.Thread.ID == "" || result.Model == "" {
			run.protocolErr = errors.New("Codex app-server returned an incomplete thread")
			return
		}
		run.threadID = result.Thread.ID
		// The server resolves aliases such as "auto" to the concrete model.
		// Carry that model into the completion usage persisted for rebuilds.
		run.eventParser.req.Model = result.Model
		if result.Thread.ID != run.req.ResumeID {
			run.emit(agent.Event{
				T:              time.Now().UnixMilli(),
				Type:           agent.EventSessionUpdated,
				Provider:       agent.ProviderCodex,
				ConversationID: run.req.ConversationID,
				SessionID:      result.Thread.ID,
				Model:          result.Model,
			})
		}
		run.protocolErr = run.write(map[string]any{
			"method": "turn/start",
			"id":     appServerTurnRequestID,
			"params": buildAppServerTurnParams(run.req, result.Thread.ID, result.Model),
		})

	case appServerTurnRequestID:
		var result appServerTurnStartResult
		if err := json.Unmarshal(resultJSON, &result); err != nil {
			run.protocolErr = fmt.Errorf("decode Codex turn response: %w", err)
			return
		}
		if result.Turn.ID == "" {
			run.protocolErr = errors.New("Codex app-server returned a turn without an id")
			return
		}
		run.turnID = result.Turn.ID
		pending := run.pendingNotifications
		run.pendingNotifications = nil
		for _, notification := range pending {
			if run.terminal {
				break
			}
			run.handleNotification(notification)
		}
	}
}

func (run *appServerRun) finish() error {
	run.cancelAllRequests()
	run.requestWG.Wait()
	select {
	case requestErr := <-run.requestErr:
		if run.protocolErr == nil {
			run.protocolErr = requestErr
		}
	default:
	}
	if scanErr := run.scanner.Err(); scanErr != nil && run.ctx.Err() == nil && run.protocolErr == nil {
		run.protocolErr = fmt.Errorf("Codex app-server stdout: %w", scanErr)
	}
	if run.protocolErr != nil || !run.terminal {
		_ = run.closeInput()
		if run.cmd.Process != nil {
			_ = run.cmd.Process.Kill()
		}
	}
	waitErr := run.cmd.Wait()
	stderrText := <-run.stderrDone

	if errors.Is(run.ctx.Err(), context.Canceled) {
		return nil
	}
	if run.protocolErr != nil {
		return &agentruntime.ProcessError{Err: run.protocolErr, Stderr: stderrText}
	}
	if run.runFailed {
		return &agentruntime.ProcessError{Err: agent.ErrRunFailed, Stderr: stderrText}
	}
	if !run.terminal {
		if waitErr == nil {
			waitErr = errors.New("Codex app-server closed before the turn completed")
		}
		return &agentruntime.ProcessError{Err: waitErr, Stderr: stderrText}
	}
	return nil
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
