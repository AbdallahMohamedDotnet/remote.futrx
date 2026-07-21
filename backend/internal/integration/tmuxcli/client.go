package tmuxcli

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"os/exec"
	"strconv"
	"strings"

	servicetmux "github.com/futrx-com/remote.futrx.com/internal/service/tmux"
)

type Session = servicetmux.Session

type Client struct{}

func New() *Client {
	return &Client{}
}

func ValidName(name string) bool {
	return servicetmux.ValidName(name)
}

// All session names are pre-validated against validName before being passed to
// tmux as argv (no shell). No injection surface.
func (c *Client) List() []Session {
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

func (c *Client) Create(name string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name).Run()
}

func (c *Client) Kill(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

func (c *Client) Has(name string) bool {
	for _, s := range c.List() {
		if s.Name == name {
			return true
		}
	}
	return false
}

// Cwd returns the active pane's current working directory for a session.
func (c *Client) Cwd(session string) (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "-t", session,
		"-F", "#{pane_current_path}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SendText loads text into a named tmux paste buffer and pastes it into the
// target session, then optionally sends an Enter keystroke. This is the
// canonical "send a chat message to a tmux session" pattern: tmux owns the
// bracketed-paste wrapping, the Enter is a real key event delivered after the
// paste completes, and the call works whether or not anyone is attached.
func (c *Client) SendText(session, text string, pressEnter bool) error {
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
