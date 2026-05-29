package claudelogin

// Manager owns the single interactive `claude auth login --claudeai` process.
// HTTP handlers stay in transport/http; this package only deals with process
// lifecycle, PTY IO, and credential detection.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	claudeLoginTimeout = 10 * time.Minute
	claudeURLReadWait  = 15 * time.Second
	claudeExitWait     = 30 * time.Second
)

var (
	ErrCodeRequired   = errors.New("code is required")
	ErrNoSession      = errors.New("no login session in progress - call /api/claude/login/start first")
	ErrClaudeNotFound = errors.New("claude CLI not found on PATH - install it first")

	claudeAuthURLRe = regexp.MustCompile(`https://claude\.com/cai/oauth/authorize\?[^\s]+`)
)

type Manager struct {
	mu      sync.Mutex
	session *loginSession
}

type StartResult struct {
	URL     string
	Resumed bool
}

type loginSession struct {
	cmd       *exec.Cmd
	ptmx      *os.File
	startedAt time.Time
	cancel    context.CancelFunc
	done      chan struct{}

	mu      sync.Mutex
	url     string
	output  strings.Builder
	exitErr error
}

func New() *Manager {
	return &Manager{}
}

func (m *Manager) Authenticated() bool {
	return authenticated()
}

func (m *Manager) Start(ctx context.Context) (StartResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	if m.session != nil {
		select {
		case <-m.session.done:
			m.session = nil
		default:
			result := StartResult{URL: m.session.URL(), Resumed: true}
			m.mu.Unlock()
			return result, nil
		}
	}

	if _, err := exec.LookPath("claude"); err != nil {
		m.mu.Unlock()
		return StartResult{}, ErrClaudeNotFound
	}

	loginCtx, cancel := context.WithTimeout(context.Background(), claudeLoginTimeout)
	cmd := exec.CommandContext(loginCtx, "claude", "auth", "login", "--claudeai")
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		m.mu.Unlock()
		return StartResult{}, fmt.Errorf("pty start: %w", err)
	}

	sess := &loginSession{
		cmd:       cmd,
		ptmx:      ptmx,
		startedAt: time.Now(),
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	m.session = sess
	m.mu.Unlock()

	urlFound := make(chan string, 1)
	go readLoginOutput(sess, urlFound)
	go waitForLoginExit(sess)

	select {
	case url := <-urlFound:
		return StartResult{URL: url}, nil
	case <-time.After(claudeURLReadWait):
		cancel()
		m.clear(sess)
		return StartResult{}, fmt.Errorf("did not see Anthropic OAuth URL within %s; first 500 bytes of claude output: %s",
			claudeURLReadWait, truncate(sess.Output(), 500))
	case <-sess.done:
		m.clear(sess)
		msg := "claude exited before printing OAuth URL"
		if exitErr := sess.ExitErr(); exitErr != nil {
			msg += " (" + exitErr.Error() + ")"
		}
		return StartResult{}, fmt.Errorf("%s; output: %s", msg, truncate(sess.Output(), 500))
	case <-ctx.Done():
		cancel()
		m.clear(sess)
		return StartResult{}, ctx.Err()
	}
}

func (m *Manager) SubmitCode(ctx context.Context, code string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrCodeRequired
	}

	sess := m.current()
	if sess == nil {
		return ErrNoSession
	}

	if _, err := io.WriteString(sess.ptmx, code+"\r"); err != nil {
		return fmt.Errorf("write to claude stdin: %w", err)
	}

	select {
	case <-sess.done:
	case <-time.After(claudeExitWait):
		sess.cancel()
		<-sess.done
		m.clear(sess)
		return fmt.Errorf("claude did not exit within %s after code paste; last output: %s",
			claudeExitWait, truncate(sess.Output(), 500))
	case <-ctx.Done():
		return ctx.Err()
	}

	debugOut := sess.Output()
	exitErr := sess.ExitErr()
	m.clear(sess)

	if exitErr != nil && !errors.Is(exitErr, context.Canceled) {
		return fmt.Errorf("claude exited with error: %w; output: %s", exitErr, truncate(debugOut, 500))
	}
	if !authenticated() {
		return fmt.Errorf("claude exited cleanly but no credentials file was written; output: %s",
			truncate(debugOut, 500))
	}
	return nil
}

func (m *Manager) Cancel(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sess := m.current()
	if sess == nil {
		return nil
	}
	sess.cancel()

	select {
	case <-sess.done:
		m.clear(sess)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) current() *loginSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session
}

func (m *Manager) clear(sess *loginSession) {
	m.mu.Lock()
	if m.session == sess {
		m.session = nil
	}
	m.mu.Unlock()
}

func readLoginOutput(sess *loginSession, urlFound chan<- string) {
	defer func() { _ = sess.ptmx.Close() }()

	reader := bufio.NewReader(sess.ptmx)
	buf := make([]byte, 4096)
	for {
		n, rerr := reader.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			output := sess.AppendOutput(chunk)
			if sess.URL() == "" {
				if match := claudeAuthURLRe.FindString(output); match != "" {
					sess.SetURL(match)
					select {
					case urlFound <- match:
					default:
					}
				}
			}
		}
		if rerr != nil {
			return
		}
	}
}

func waitForLoginExit(sess *loginSession) {
	sess.SetExitErr(sess.cmd.Wait())
	close(sess.done)
}

func (s *loginSession) AppendOutput(chunk string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.output.WriteString(chunk)
	return s.output.String()
}

func (s *loginSession) Output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.output.String()
}

func (s *loginSession) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.url
}

func (s *loginSession) SetURL(url string) {
	s.mu.Lock()
	s.url = url
	s.mu.Unlock()
}

func (s *loginSession) ExitErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitErr
}

func (s *loginSession) SetExitErr(err error) {
	s.mu.Lock()
	s.exitErr = err
	s.mu.Unlock()
}

func authenticated() bool {
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
