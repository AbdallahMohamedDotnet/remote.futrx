package main

// Web-bridged `claude auth login --claudeai` flow.
//
// The login is interactive (claude prints an Anthropic OAuth URL, then waits
// for the user to paste back a code). On a headless server there's no browser
// to open, but the URL is still printed. We:
//   1. Spawn claude in a PTY so it actually prints the URL and prompt.
//   2. Stream stdout, regex-match the URL, surface it to the frontend.
//   3. Accept a paste-code from the frontend, write it to claude's stdin.
//   4. Wait for claude to exit, then confirm credentials landed on disk.
//
// Only one login can be in flight at a time per server (single global
// instance) — there's one ~/.claude/ on this host and one admin user.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

const (
	claudeLoginTimeout  = 10 * time.Minute // user has this long to paste a code
	claudeURLReadTimout = 15 * time.Second // claude must print the URL within this
	claudeExitWait      = 30 * time.Second // after submitting code, wait for exit
)

var claudeAuthURLRe = regexp.MustCompile(`https://claude\.com/cai/oauth/authorize\?[^\s]+`)

// ClaudeLogin manages a single in-flight `claude auth login` session.
type ClaudeLogin struct {
	mu      sync.Mutex
	session *claudeLoginSession
}

type claudeLoginSession struct {
	cmd       *exec.Cmd
	ptmx      *os.File
	url       string
	output    *strings.Builder // accumulated stdout for debugging
	startedAt time.Time
	cancel    context.CancelFunc
	done      chan struct{} // closed when process exits
	exitErr   error         // populated before `done` closes
}

func NewClaudeLogin() *ClaudeLogin { return &ClaudeLogin{} }

// claudeAuthenticated returns true if a credentials file exists.
// Conservative — doesn't validate the token is still good. The chat-stream
// surfaces the runtime auth-failed error if a stale token slipped through.
func claudeAuthenticated() bool {
	for _, name := range []string{".credentials.json", "credentials.json"} {
		if _, err := os.Stat(filepath.Join(claudeHomeDir(), name)); err == nil {
			return true
		}
	}
	return false
}

func claudeHomeDir() string {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("HOME"); v != "" {
		return filepath.Join(v, ".claude")
	}
	return "/root/.claude"
}

// --- HTTP handlers ---------------------------------------------------------

func (c *ClaudeLogin) handleStatus(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, map[string]any{
		"authenticated": claudeAuthenticated(),
	})
}

func (c *ClaudeLogin) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErr(w, 405, "method not allowed")
		return
	}

	c.mu.Lock()
	// If there's already a session, return its URL (idempotent) unless it has
	// already finished — in which case clear it and start fresh.
	if c.session != nil {
		select {
		case <-c.session.done:
			c.session = nil // exited; allow restart
		default:
			url := c.session.url
			c.mu.Unlock()
			sendJSON(w, 200, map[string]any{"url": url, "resumed": true})
			return
		}
	}

	if _, err := exec.LookPath("claude"); err != nil {
		c.mu.Unlock()
		sendErr(w, 500, "claude CLI not found on PATH — install it first")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), claudeLoginTimeout)
	cmd := exec.CommandContext(ctx, "claude", "auth", "login", "--claudeai")
	// Inherit env so $HOME / $CLAUDE_CONFIG_DIR resolve correctly.
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		c.mu.Unlock()
		sendErr(w, 500, "pty start: "+err.Error())
		return
	}

	sess := &claudeLoginSession{
		cmd:       cmd,
		ptmx:      ptmx,
		output:    &strings.Builder{},
		startedAt: time.Now(),
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	c.session = sess
	c.mu.Unlock()

	// Reader goroutine: pull bytes off the pty, look for the URL, accumulate
	// everything else for debugging.
	urlFound := make(chan string, 1)
	go func() {
		defer func() { _ = ptmx.Close() }()
		reader := bufio.NewReader(ptmx)
		buf := make([]byte, 4096)
		for {
			n, rerr := reader.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				sess.output.WriteString(chunk)
				if sess.url == "" {
					if m := claudeAuthURLRe.FindString(sess.output.String()); m != "" {
						sess.url = m
						select {
						case urlFound <- m:
						default:
						}
					}
				}
			}
			if rerr != nil {
				break
			}
		}
	}()

	// Waiter goroutine: when the process exits, record the error and signal.
	go func() {
		sess.exitErr = cmd.Wait()
		close(sess.done)
	}()

	// Wait up to claudeURLReadTimout for the URL to surface.
	select {
	case url := <-urlFound:
		sendJSON(w, 200, map[string]any{"url": url})
	case <-time.After(claudeURLReadTimout):
		// Couldn't extract the URL. Kill the process to free the lock and
		// return what we captured for debugging.
		cancel()
		c.mu.Lock()
		debugOut := sess.output.String()
		c.session = nil
		c.mu.Unlock()
		sendErr(w, 500, "did not see Anthropic OAuth URL within "+
			claudeURLReadTimout.String()+
			"; first 500 bytes of claude output: "+truncate(debugOut, 500))
	case <-sess.done:
		// Process exited before printing the URL — claude is broken or
		// installed wrong. Bubble up whatever it said.
		c.mu.Lock()
		debugOut := sess.output.String()
		c.session = nil
		c.mu.Unlock()
		msg := "claude exited before printing OAuth URL"
		if sess.exitErr != nil {
			msg += " (" + sess.exitErr.Error() + ")"
		}
		sendErr(w, 500, msg+"; output: "+truncate(debugOut, 500))
	}
}

func (c *ClaudeLogin) handleCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErr(w, 405, "method not allowed")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := readJSONBody(r, &body); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	code := strings.TrimSpace(body.Code)
	if code == "" {
		sendErr(w, 400, "code is required")
		return
	}

	c.mu.Lock()
	sess := c.session
	c.mu.Unlock()
	if sess == nil {
		sendErr(w, 400, "no login session in progress — call /api/claude/login/start first")
		return
	}

	// Send the code as if the user typed it + Enter.
	if _, err := io.WriteString(sess.ptmx, code+"\r"); err != nil {
		sendErr(w, 500, "write to claude stdin: "+err.Error())
		return
	}

	// Wait for claude to exit. Success = exit 0 AND credentials file exists.
	select {
	case <-sess.done:
		// Drained.
	case <-time.After(claudeExitWait):
		// Stuck — kill it and report.
		sess.cancel()
		<-sess.done
		c.mu.Lock()
		c.session = nil
		c.mu.Unlock()
		sendErr(w, 500, "claude did not exit within "+claudeExitWait.String()+
			" after code paste; last output: "+truncate(sess.output.String(), 500))
		return
	}

	c.mu.Lock()
	debugOut := sess.output.String()
	exitErr := sess.exitErr
	c.session = nil
	c.mu.Unlock()

	if exitErr != nil && !errors.Is(exitErr, context.Canceled) {
		sendErr(w, 500, "claude exited with error: "+exitErr.Error()+
			"; output: "+truncate(debugOut, 500))
		return
	}
	if !claudeAuthenticated() {
		sendErr(w, 500, "claude exited cleanly but no credentials file was written; output: "+
			truncate(debugOut, 500))
		return
	}
	sendJSON(w, 200, map[string]any{"success": true})
}

func (c *ClaudeLogin) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErr(w, 405, "method not allowed")
		return
	}
	c.mu.Lock()
	if c.session != nil {
		c.session.cancel()
		<-c.session.done
		c.session = nil
	}
	c.mu.Unlock()
	sendJSON(w, 200, map[string]bool{"ok": true})
}

// readJSONBody is a small helper that pairs nicely with sendErr/sendJSON.
func readJSONBody(r *http.Request, v any) error {
	const max = 1 << 16
	body := http.MaxBytesReader(nil, r.Body, max)
	defer body.Close()
	if err := json.NewDecoder(body).Decode(v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}
