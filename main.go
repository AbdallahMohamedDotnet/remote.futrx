// remote.futrx.dev: self-hosted Claude Code chat + terminal-PTY server.
//
// Backend serves:
//   - Static SPA (Preact/Vite bundle) embedded via go:embed
//   - HTTP API for chat metadata + per-chat upload
//   - WS /ws for tmux PTY streaming (terminal SSH bridge, no UI surfaces it)
//   - WS /ws/chat/{id} for claude streaming (stream-json + partial messages)
//
// Frontend (web/) is a single chat UI that drives `claude -p` with per-session
// resume, markdown rendering, tool-call widgets, and an AskUserQuestion wizard.

package main

import (
	"context"
	crand "crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

//go:embed public
var publicFS embed.FS

var (
	validName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)
	upgrader  = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		// We're behind Caddy with cookie auth; Caddy enforces same-origin
		// by virtue of the cookie scope. No need to double-check here.
		CheckOrigin: func(*http.Request) bool { return true },
	}
)

type Session struct {
	Name     string `json:"name"`
	Created  int64  `json:"created"`
	Attached bool   `json:"attached"`
	Windows  int    `json:"windows"`
	Cwd      string `json:"cwd"`
}

// --- tmux ------------------------------------------------------------------
// All session names are pre-validated against validName before being passed
// to tmux as argv (no shell). No injection surface.

func tmuxList() []Session {
	out, err := exec.Command("tmux", "list-sessions", "-F",
		"#{session_name}|#{session_created}|#{session_attached}|#{session_windows}|#{pane_current_path}").Output()
	if err != nil {
		return []Session{}
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return []Session{}
	}
	sessions := make([]Session, 0)
	for _, line := range strings.Split(s, "\n") {
		// Path may legitimately contain "|", so cap the split.
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 4 {
			continue
		}
		created, _ := strconv.ParseInt(parts[1], 10, 64)
		attached, _ := strconv.Atoi(parts[2])
		windows, _ := strconv.Atoi(parts[3])
		cwd := ""
		if len(parts) == 5 {
			cwd = parts[4]
		}
		sessions = append(sessions, Session{
			Name:     parts[0],
			Created:  created * 1000,
			Attached: attached > 0,
			Windows:  windows,
			Cwd:      cwd,
		})
	}
	return sessions
}

func tmuxCreate(name string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name).Run()
}

func tmuxKill(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

func tmuxHas(name string) bool {
	for _, s := range tmuxList() {
		if s.Name == name {
			return true
		}
	}
	return false
}

// tmuxCwd returns the active pane's current working directory for a session.
func tmuxCwd(session string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "-t", session,
		"-F", "#{pane_current_path}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// tmuxSendText loads `text` into a named tmux paste buffer and pastes it into
// the target session, then optionally sends an Enter keystroke. This is the
// canonical "send a chat message to a tmux session" pattern: tmux owns the
// bracketed-paste wrapping, the Enter is a real key event delivered after the
// paste completes, and the call works whether or not anyone is attached.
func tmuxSendText(session, text string, pressEnter bool) error {
	var nonce [8]byte
	if _, err := crand.Read(nonce[:]); err != nil {
		return err
	}
	buf := "send-" + hex.EncodeToString(nonce[:])

	load := exec.Command("tmux", "load-buffer", "-b", buf, "-")
	load.Stdin = strings.NewReader(text)
	if out, err := load.CombinedOutput(); err != nil {
		return errors.New("load-buffer: " + strings.TrimSpace(string(out)))
	}

	if out, err := exec.Command("tmux", "paste-buffer", "-b", buf, "-t", session).CombinedOutput(); err != nil {
		_ = exec.Command("tmux", "delete-buffer", "-b", buf).Run()
		return errors.New("paste-buffer: " + strings.TrimSpace(string(out)))
	}
	_ = exec.Command("tmux", "delete-buffer", "-b", buf).Run()

	if pressEnter {
		if out, err := exec.Command("tmux", "send-keys", "-t", session, "Enter").CombinedOutput(); err != nil {
			return errors.New("send-keys: " + strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// --- HTTP API --------------------------------------------------------------

func sendJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func sendErr(w http.ResponseWriter, status int, msg string) {
	sendJSON(w, status, map[string]string{"error": msg})
}

func apiSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sendJSON(w, 200, tmuxList())
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil {
			sendErr(w, 400, "invalid json")
			return
		}
		name := strings.TrimSpace(body.Name)
		if !validName.MatchString(name) {
			sendErr(w, 400, "invalid name (alphanumeric, _ -, 1-32 chars)")
			return
		}
		if tmuxHas(name) {
			sendErr(w, 409, "session exists")
			return
		}
		if err := tmuxCreate(name); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		sendJSON(w, 201, map[string]string{"name": name})
	default:
		sendErr(w, 405, "method not allowed")
	}
}

func apiSession(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.SplitN(rest, "/", 2)
	name := parts[0]
	if !validName.MatchString(name) {
		sendErr(w, 400, "invalid name")
		return
	}

	// /api/sessions/{name}/upload — multipart upload(s) into the session's cwd.
	if len(parts) == 2 && parts[1] == "upload" {
		if r.Method != http.MethodPost {
			sendErr(w, 405, "method not allowed")
			return
		}
		if !tmuxHas(name) {
			sendErr(w, 404, "session not found")
			return
		}
		apiSessionUpload(name, w, r)
		return
	}

	// /api/sessions/{name}/send — POST a chat message into the tmux session
	if len(parts) == 2 && parts[1] == "send" {
		if r.Method != http.MethodPost {
			sendErr(w, 405, "method not allowed")
			return
		}
		if !tmuxHas(name) {
			sendErr(w, 404, "session not found")
			return
		}
		var body struct {
			Text       string `json:"text"`
			PressEnter *bool  `json:"pressEnter,omitempty"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			sendErr(w, 400, "invalid json")
			return
		}
		pressEnter := true
		if body.PressEnter != nil {
			pressEnter = *body.PressEnter
		}
		if body.Text == "" && !pressEnter {
			sendJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		if err := tmuxSendText(name, body.Text, pressEnter); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		sendJSON(w, 200, map[string]bool{"ok": true})
		return
	}

	// /api/sessions/{name} — DELETE
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			sendErr(w, 405, "method not allowed")
			return
		}
		if !tmuxHas(name) {
			sendErr(w, 404, "not found")
			return
		}
		if err := tmuxKill(name); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		sendJSON(w, 200, map[string]bool{"ok": true})
		return
	}

	sendErr(w, 404, "not found")
}

// apiSessionUpload accepts one or more files via multipart and writes them
// into the session's current pane working directory. Caller has already
// validated `name` and confirmed the session exists.
func apiSessionUpload(name string, w http.ResponseWriter, r *http.Request) {
	cwd, err := tmuxCwd(name)
	if err != nil || cwd == "" {
		sendErr(w, 500, "could not resolve session cwd")
		return
	}
	handleMultipartUpload(cwd, w, r)
}

// handleMultipartUpload reads files from a multipart form (field name "files")
// and writes each to cwd, refusing to overwrite. Used by both tmux-session
// upload and chat upload.
func handleMultipartUpload(cwd string, w http.ResponseWriter, r *http.Request) {
	const maxTotal = 200 << 20 // 200 MiB per request
	r.Body = http.MaxBytesReader(w, r.Body, maxTotal)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		sendErr(w, 400, "upload too large or malformed: "+err.Error())
		return
	}
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		sendErr(w, 500, "cwd is not a directory: "+cwd)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		sendErr(w, 400, "no files in request (field name: files)")
		return
	}

	type result struct {
		Name  string `json:"name"`
		Path  string `json:"path,omitempty"`
		Size  int64  `json:"size,omitempty"`
		Error string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(files))

	for _, fh := range files {
		res := result{Name: fh.Filename}
		base := filepath.Base(fh.Filename)
		if base == "" || base == "." || base == ".." ||
			strings.ContainsAny(base, "/\\") {
			res.Error = "invalid filename"
			results = append(results, res)
			continue
		}
		dest := filepath.Join(cwd, base)
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				res.Error = "file already exists at " + dest
			} else {
				res.Error = err.Error()
			}
			results = append(results, res)
			continue
		}
		src, err := fh.Open()
		if err != nil {
			out.Close()
			os.Remove(dest)
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		n, err := io.Copy(out, src)
		src.Close()
		_ = out.Close()
		if err != nil {
			os.Remove(dest)
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		res.Path = dest
		res.Size = n
		results = append(results, res)
	}

	sendJSON(w, 200, map[string]any{"cwd": cwd, "results": results})
}

// --- WebSocket / PTY -------------------------------------------------------

type clientMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("session")
	if !validName.MatchString(name) {
		http.Error(w, "invalid session name", http.StatusBadRequest)
		return
	}
	if !tmuxHas(name) {
		if err := tmuxCreate(name); err != nil {
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
	}

	// -d kicks any other attached client so a phone reconnect takes over cleanly.
	cmd := exec.Command("tmux", "attach-session", "-d", "-t", name)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		http.Error(w, "pty failed", http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			cancel()
			_ = ptmx.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
			_ = conn.Close()
		})
	}
	defer cleanup()

	// Single writer to ws (this goroutine reads from PTY, writes to ws).
	// gorilla/websocket forbids concurrent writes, so all WS writes happen here.
	go func() {
		defer cleanup()
		buf := make([]byte, 8192)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Reader: WS -> PTY (+ control messages).
	conn.SetReadLimit(1 << 20)
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		switch mt {
		case websocket.BinaryMessage:
			if _, err := ptmx.Write(data); err != nil {
				return
			}
		case websocket.TextMessage:
			var msg clientMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "input":
				if _, err := ptmx.Write([]byte(msg.Data)); err != nil {
					return
				}
			case "resize":
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
			}
		}
	}
}

// --- main ------------------------------------------------------------------

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	host := envDefault("HOST", "172.18.0.1")
	port := envDefault("PORT", "7682")
	dataDir := envDefault("DATA_DIR", "/opt/remote.futrx.dev/data")

	chatStore, err := NewChatStore(dataDir)
	if err != nil {
		log.Fatalf("init chat store: %v", err)
	}

	static, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatal(err)
	}
	staticHandler := http.FileServer(http.FS(static))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", apiSessions)
	mux.HandleFunc("/api/sessions/", apiSession)
	mux.HandleFunc("/api/chats", chatStore.handleChatsCollection)
	mux.HandleFunc("/api/chats/", chatStore.handleChatResource)
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/ws/chat/", func(w http.ResponseWriter, r *http.Request) {
		chatStreamHandler(chatStore, w, r)
	})
	mux.Handle("/", staticHandler)

	addr := host + ":" + port
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: long-lived WebSockets.
	}
	log.Printf("remote.futrx.dev listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
