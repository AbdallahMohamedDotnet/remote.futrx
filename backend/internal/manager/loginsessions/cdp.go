package loginsessions

// cdp.go: a tiny CDP (Chrome DevTools Protocol) client tailored to the
// login-session use case. It is NOT a full CDP client; it supports just
// the handful of commands and events we need:
//   - Target.setAutoAttach / Target.attachToTarget (find the page)
//   - Page.startScreencast / Page.screencastFrameAck (live view)
//   - Input.dispatchMouseEvent / dispatchKeyEvent / insertText (drive it)
//   - Page.navigate (URL bar)
//   - Network.getAllCookies + Runtime.evaluate (capture in milestone 3)
//
// We use the gorilla/websocket package's blocking ReadJSON / WriteJSON,
// with a single writer goroutine fronted by an outgoing channel so the
// caller doesn't need to think about concurrency. All commands wait for
// the matching id-tagged response via an in-flight map.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// CDPClient is a single CDP connection. Not safe for use after Close().
type CDPClient struct {
	conn *websocket.Conn

	mu       sync.Mutex
	nextID   atomic.Int64
	pending  map[int64]chan cdpResponse
	handlers []EventHandler

	writeCh chan []byte
	closed  chan struct{}
	closeMu sync.Once
}

type cdpResponse struct {
	Result json.RawMessage
	Err    *cdpError
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *cdpError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("cdp error %d: %s", e.Code, e.Message)
}

// EventHandler is called for every untyped CDP event (no id, has method).
// sessionID may be empty for browser-level events.
type EventHandler func(method, sessionID string, params json.RawMessage)

// DialCDP opens a websocket to the chromium DevTools endpoint at
// http://target/json/version, reads webSocketDebuggerUrl, and dials it.
func DialCDP(ctx context.Context, target string) (*CDPClient, error) {
	version, err := FetchVersion(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("fetch /json/version: %w", err)
	}
	wsURL, _ := version["webSocketDebuggerUrl"].(string)
	if wsURL == "" {
		return nil, errors.New("missing webSocketDebuggerUrl in /json/version")
	}

	// Chromium reports the URL using its self-reported hostname (often
	// "localhost"). We need to rewrite it to point at the container's
	// outward-facing host:port so we actually reach it from the host.
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("parse ws url: %w", err)
	}
	u.Host = target

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial cdp ws %s: %w", u.String(), err)
	}

	c := &CDPClient{
		conn:    conn,
		pending: map[int64]chan cdpResponse{},
		writeCh: make(chan []byte, 32),
		closed:  make(chan struct{}),
	}

	go c.readLoop()
	go c.writeLoop()
	return c, nil
}

// On registers an event handler. Handlers fire in registration order. The
// caller MUST NOT block inside a handler — fan out to its own channels.
func (c *CDPClient) On(h EventHandler) {
	c.mu.Lock()
	c.handlers = append(c.handlers, h)
	c.mu.Unlock()
}

// Send issues a CDP command on the root (browser) session and waits for
// the response. Returns the raw result bytes.
func (c *CDPClient) Send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.SendOn(ctx, "", method, params)
}

// SendOn issues a CDP command on a specific page session (use "" for the
// browser-level session).
func (c *CDPClient) SendOn(ctx context.Context, sessionID, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	envelope := map[string]any{
		"id":     id,
		"method": method,
	}
	if params != nil {
		envelope["params"] = params
	}
	if sessionID != "" {
		envelope["sessionId"] = sessionID
	}
	buf, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}

	ch := make(chan cdpResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	select {
	case c.writeCh <- buf:
	case <-c.closed:
		return nil, errors.New("cdp client closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case resp := <-ch:
		if resp.Err != nil {
			return nil, resp.Err
		}
		return resp.Result, nil
	case <-c.closed:
		return nil, errors.New("cdp client closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close shuts the connection. Idempotent.
func (c *CDPClient) Close() {
	c.closeMu.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
	})
}

func (c *CDPClient) writeLoop() {
	for {
		select {
		case <-c.closed:
			return
		case buf := <-c.writeCh:
			if err := c.conn.WriteMessage(websocket.TextMessage, buf); err != nil {
				c.Close()
				return
			}
		}
	}
}

func (c *CDPClient) readLoop() {
	defer c.Close()
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var env struct {
			ID        int64           `json:"id"`
			Method    string          `json:"method"`
			Params    json.RawMessage `json:"params"`
			Result    json.RawMessage `json:"result"`
			Error     *cdpError       `json:"error"`
			SessionID string          `json:"sessionId"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.ID != 0 {
			c.mu.Lock()
			ch, ok := c.pending[env.ID]
			c.mu.Unlock()
			if ok {
				ch <- cdpResponse{Result: env.Result, Err: env.Error}
			}
			continue
		}
		// Event.
		c.mu.Lock()
		handlers := append([]EventHandler(nil), c.handlers...)
		c.mu.Unlock()
		for _, h := range handlers {
			h(env.Method, env.SessionID, env.Params)
		}
	}
}

// TargetInfo subset of CDP Target.TargetInfo.
type TargetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Attached bool   `json:"attached"`
}

// GetTargets returns Target.getTargets results.
func (c *CDPClient) GetTargets(ctx context.Context) ([]TargetInfo, error) {
	raw, err := c.Send(ctx, "Target.getTargets", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		TargetInfos []TargetInfo `json:"targetInfos"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.TargetInfos, nil
}

// PageTargetID picks the first "page" type target from a list.
func PageTargetID(targets []TargetInfo) string {
	for _, t := range targets {
		if t.Type == "page" {
			return t.TargetID
		}
	}
	return ""
}

// FindPageSessionID attaches to a page target and returns the resulting
// sessionId. Uses flatten so events all arrive on the root socket.
func (c *CDPClient) FindPageSessionID(ctx context.Context, targetID string) (string, error) {
	raw, err := c.Send(ctx, "Target.attachToTarget", map[string]any{
		"targetId": targetID,
		"flatten":  true,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.SessionID == "" {
		return "", errors.New("attachToTarget returned empty sessionId")
	}
	return out.SessionID, nil
}

// StripQuery is a defensive helper used when relaying URLs from the
// frontend; trims any whitespace and disallows obvious shell-meta.
func StripQuery(s string) string {
	return strings.TrimSpace(s)
}
