package wstransport

// login_session_socket.go: bridge between the browser-side <canvas> view
// of a Chromium login session and the headed Chromium running inside the
// project's LXD container. We connect to Chromium over CDP, ask it for
// screencast frames, and forward client mouse/keyboard input back the
// other way. The session itself is owned by the loginsessions.Manager;
// this socket only consumes it.

import (
	"time"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	loginsessions "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/manager/loginsessions"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
	"github.com/gorilla/websocket"
)

// LoginSessionLookup returns the session by id (or nil, false if missing).
type LoginSessionLookup interface {
	Get(id string) (*loginsessions.Session, bool)
}

type LoginSessionSocket struct {
	manager LoginSessionLookup
	access  ProjectAccessChecker
}

func NewLoginSessionSocket(manager LoginSessionLookup) *LoginSessionSocket {
	return &LoginSessionSocket{manager: manager}
}

func (s *LoginSessionSocket) WithAccessChecker(access ProjectAccessChecker) *LoginSessionSocket {
	s.access = access
	return s
}

func (s *LoginSessionSocket) RegisterRoutes(mux *http.ServeMux, upgrader websocket.Upgrader) {
	mux.HandleFunc("/ws/login-session/", s.Handle(upgrader))
}

func (s *LoginSessionSocket) Handle(upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.handle(upgrader, w, r)
	}
}

func (s *LoginSessionSocket) handle(upgrader websocket.Upgrader, w http.ResponseWriter, r *http.Request) {
	if s.manager == nil {
		http.Error(w, "login sessions unavailable", http.StatusServiceUnavailable)
		return
	}
	sid := strings.TrimPrefix(r.URL.Path, "/ws/login-session/")
	if sid == "" || strings.Contains(sid, "/") {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}
	sess, ok := s.manager.Get(sid)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if s.access != nil {
		email, isAdmin, err := s.access.CallerAndAdmin(r.Context(), r)
		if err != nil || email == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if !isAdmin {
			ok, err := s.access.HasAccess(r.Context(), serviceproject.ID(sess.ProjectID), email)
			if err != nil || !ok {
				http.Error(w, "not a member of this project", http.StatusForbidden)
				return
			}
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Single-writer guard — all WS writes happen through writeJSON which
	// holds connMu. Per gorilla/websocket docs, concurrent writes panic.
	var connMu sync.Mutex
	writeJSON := func(v any) error {
		connMu.Lock()
		defer connMu.Unlock()
		return conn.WriteJSON(v)
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer conn.Close()

	cdp, err := loginsessions.DialCDP(ctx, sess.DevToolsAddr(), sess.HostHeader())
	if err != nil {
		_ = writeJSON(map[string]any{"type": "error", "message": err.Error()})
		return
	}
	defer cdp.Close()

	// Find the page target and attach.
	targets, err := cdp.GetTargets(ctx)
	if err != nil {
		_ = writeJSON(map[string]any{"type": "error", "message": "list targets: " + err.Error()})
		return
	}
	targetID := loginsessions.PageTargetID(targets)
	if targetID == "" {
		_ = writeJSON(map[string]any{"type": "error", "message": "no page target found"})
		return
	}
	pageSession, err := cdp.FindPageSessionID(ctx, targetID)
	if err != nil {
		_ = writeJSON(map[string]any{"type": "error", "message": "attach: " + err.Error()})
		return
	}

	// Enable Page domain (required for screencast) on this page session.
	if _, err := cdp.SendOn(ctx, pageSession, "Page.enable", nil); err != nil {
		_ = writeJSON(map[string]any{"type": "error", "message": "Page.enable: " + err.Error()})
		return
	}

	// Force the viewport to a known size. Headless Chromium defaults
	// to 800x600, which makes the frontend's click translation (which
	// scales DOM coordinates against a 1280x720 native canvas) land
	// outside the rendered page entirely. Aligning the viewport to
	// match the frontend's NATIVE_* constants makes clicks hit.
	_, _ = cdp.SendOn(ctx, pageSession, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width":             1280,
		"height":            720,
		"deviceScaleFactor": 1,
		"mobile":            false,
	})

	// Page.frameNavigated -> emit a "url" message so the URL bar updates.
	cdp.On(func(method, sessionID string, params json.RawMessage) {
		if sessionID != pageSession || method != "Page.frameNavigated" {
			return
		}
		var p struct {
			Frame struct {
				URL    string `json:"url"`
				Parent string `json:"parentId"`
			} `json:"frame"`
		}
		if err := json.Unmarshal(params, &p); err == nil && p.Frame.Parent == "" {
			_ = writeJSON(map[string]any{"type": "url", "url": p.Frame.URL})
		}
	})

	// Live view: poll Page.captureScreenshot at ~7 FPS and push each
	// JPEG to the client. Page.startScreencast does not reliably fire
	// frames for static pages in headless Chromium; captureScreenshot
	// always returns a fresh raster of the current state.
	go func() {
		t := time.NewTicker(150 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			shotCtx, shotCancel := context.WithTimeout(ctx, 3*time.Second)
			raw, err := cdp.SendOn(shotCtx, pageSession, "Page.captureScreenshot", map[string]any{
				"format":      "jpeg",
				"quality":     60,
				"captureBeyondViewport": false,
			})
			shotCancel()
			if err != nil {
				continue
			}
			var resp struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil || resp.Data == "" {
				continue
			}
			if err := writeJSON(map[string]any{
				"type":   "frame",
				"data":   resp.Data,
				"width":  1280,
				"height": 720,
			}); err != nil {
				return
			}
		}
	}()

	// Tell the client we're live + provide some initial state.
	_ = writeJSON(map[string]any{
		"type":       "ready",
		"sessionId":  sess.ID,
		"secretName": sess.SecretName,
		"url":        sess.URL,
	})

	// Read input messages from the client and translate to CDP.
	conn.SetReadLimit(1 << 20)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := handleClientMessage(ctx, cdp, pageSession, data); err != nil {
			// Best-effort surface to client; non-fatal.
			_ = writeJSON(map[string]any{"type": "warn", "message": err.Error()})
		}
	}
}

type clientLoginMsg struct {
	Type string `json:"type"`

	// Pointer
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Button string  `json:"button"`
	DX     float64 `json:"dx"`
	DY     float64 `json:"dy"`
	Click  int     `json:"clickCount"`

	// Key
	Key      string `json:"key"`
	Code     string `json:"code"`
	KeyCode  int    `json:"keyCode"`
	Mod      int    `json:"modifiers"`
	Location int    `json:"location"`
	Text     string `json:"text"`
	IsKeyDown bool  `json:"isKeyDown"`

	// Navigate
	URL string `json:"url"`
}

func handleClientMessage(ctx context.Context, cdp *loginsessions.CDPClient, pageSession string, raw []byte) error {
	var msg clientLoginMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		return errors.New("invalid json")
	}
	switch msg.Type {
	case "move":
		_, err := cdp.SendOn(ctx, pageSession, "Input.dispatchMouseEvent", map[string]any{
			"type":      "mouseMoved",
			"x":         msg.X,
			"y":         msg.Y,
			"modifiers": msg.Mod,
		})
		return err
	case "down":
		_, err := cdp.SendOn(ctx, pageSession, "Input.dispatchMouseEvent", map[string]any{
			"type":       "mousePressed",
			"x":          msg.X,
			"y":          msg.Y,
			"button":     defaultMouseButton(msg.Button),
			"clickCount": maxInt(msg.Click, 1),
			"modifiers":  msg.Mod,
		})
		return err
	case "up":
		_, err := cdp.SendOn(ctx, pageSession, "Input.dispatchMouseEvent", map[string]any{
			"type":       "mouseReleased",
			"x":          msg.X,
			"y":          msg.Y,
			"button":     defaultMouseButton(msg.Button),
			"clickCount": maxInt(msg.Click, 1),
			"modifiers":  msg.Mod,
		})
		return err
	case "click":
		// Convenience wrapper that does down+up so simple clients don't
		// have to track button-down state.
		if _, err := cdp.SendOn(ctx, pageSession, "Input.dispatchMouseEvent", map[string]any{
			"type":       "mousePressed",
			"x":          msg.X,
			"y":          msg.Y,
			"button":     defaultMouseButton(msg.Button),
			"clickCount": 1,
			"modifiers":  msg.Mod,
		}); err != nil {
			return err
		}
		_, err := cdp.SendOn(ctx, pageSession, "Input.dispatchMouseEvent", map[string]any{
			"type":       "mouseReleased",
			"x":          msg.X,
			"y":          msg.Y,
			"button":     defaultMouseButton(msg.Button),
			"clickCount": 1,
			"modifiers":  msg.Mod,
		})
		return err
	case "scroll":
		_, err := cdp.SendOn(ctx, pageSession, "Input.dispatchMouseEvent", map[string]any{
			"type":      "mouseWheel",
			"x":         msg.X,
			"y":         msg.Y,
			"deltaX":    msg.DX,
			"deltaY":    msg.DY,
			"modifiers": msg.Mod,
		})
		return err
	case "key":
		evType := "keyUp"
		if msg.IsKeyDown {
			evType = "keyDown"
		}
		_, err := cdp.SendOn(ctx, pageSession, "Input.dispatchKeyEvent", map[string]any{
			"type":      evType,
			"key":       msg.Key,
			"code":      msg.Code,
			"text":      msg.Text,
			"windowsVirtualKeyCode": msg.KeyCode,
			"modifiers": msg.Mod,
			"location":  msg.Location,
		})
		return err
	case "type":
		_, err := cdp.SendOn(ctx, pageSession, "Input.insertText", map[string]any{
			"text": msg.Text,
		})
		return err
	case "navigate":
		u := strings.TrimSpace(msg.URL)
		if u == "" {
			return errors.New("empty url")
		}
		_, err := cdp.SendOn(ctx, pageSession, "Page.navigate", map[string]any{"url": u})
		return err
	case "reload":
		_, err := cdp.SendOn(ctx, pageSession, "Page.reload", nil)
		return err
	case "back":
		_, err := cdp.SendOn(ctx, pageSession, "Page.goBack", nil)
		return err
	case "forward":
		_, err := cdp.SendOn(ctx, pageSession, "Page.goForward", nil)
		return err
	}
	return nil
}

func defaultMouseButton(b string) string {
	switch b {
	case "left", "middle", "right", "back", "forward", "none":
		return b
	default:
		return "left"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
