package claudeauth

// Service owns the single interactive `claude auth login --claudeai` process.
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

type Service struct {
	mu      sync.Mutex
	session *loginSession
	state   LoginState
	subs    map[chan Status]struct{}
}

// Status is the streamed auth snapshot, mirroring codexauth.Status so the
// frontend can consume both providers through the same shape.
type Status struct {
	Authenticated bool       `json:"authenticated"`
	Login         LoginState `json:"login,omitempty"`
}

// LoginState tracks the interactive OAuth handshake. Unlike Codex's device
// grant (which shows the user a code to type into the browser), Claude's CLI
// uses the authorization-code grant: it prints an OAuth URL and the user must
// paste a code back. AwaitingCode signals the frontend to show that input.
type LoginState struct {
	Active       bool   `json:"active"`
	AuthURL      string `json:"authUrl,omitempty"`
	AwaitingCode bool   `json:"awaitingCode,omitempty"`
	StartedAt    int64  `json:"startedAt,omitempty"`
	Completed    bool   `json:"completed,omitempty"`
	Error        string `json:"error,omitempty"`
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

func New() *Service {
	return &Service{subs: map[chan Status]struct{}{}}
}

func (s *Service) Authenticated() bool {
	return authenticated()
}

// Status returns the current auth snapshot (authenticated flag + login state).
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Service) statusLocked() Status {
	return Status{Authenticated: authenticated(), Login: s.state}
}

// Subscribe registers a status channel and immediately delivers the current
// snapshot. The returned func unsubscribes. Mirrors codexauth.Service.
func (s *Service) Subscribe() (<-chan Status, func()) {
	ch := make(chan Status, 8)
	s.mu.Lock()
	if s.subs == nil {
		s.subs = map[chan Status]struct{}{}
	}
	s.subs[ch] = struct{}{}
	status := s.statusLocked()
	s.mu.Unlock()
	ch <- status

	cancel := func() {
		s.mu.Lock()
		if _, ok := s.subs[ch]; ok {
			delete(s.subs, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, cancel
}

// Broadcast pushes the current status to every subscriber.
func (s *Service) Broadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broadcastLocked()
}

func (s *Service) broadcastLocked() {
	status := s.statusLocked()
	for ch := range s.subs {
		select {
		case ch <- status:
		default:
			delete(s.subs, ch)
			close(ch)
		}
	}
}

// setStateLocked mutates login state and notifies subscribers atomically.
func (s *Service) setStateLocked(state LoginState) {
	s.state = state
	s.broadcastLocked()
}

func (s *Service) setState(state LoginState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setStateLocked(state)
}

func (s *Service) Start(ctx context.Context) (StartResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.session != nil {
		select {
		case <-s.session.done:
			s.session = nil
		default:
			result := StartResult{URL: s.session.URL(), Resumed: true}
			s.mu.Unlock()
			return result, nil
		}
	}

	if _, err := exec.LookPath("claude"); err != nil {
		s.setStateLocked(LoginState{Error: ErrClaudeNotFound.Error()})
		s.mu.Unlock()
		return StartResult{}, ErrClaudeNotFound
	}

	loginCtx, cancel := context.WithTimeout(context.Background(), claudeLoginTimeout)
	cmd := exec.CommandContext(loginCtx, "claude", "auth", "login", "--claudeai")
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		s.setStateLocked(LoginState{Error: fmt.Sprintf("pty start: %v", err)})
		s.mu.Unlock()
		return StartResult{}, fmt.Errorf("pty start: %w", err)
	}

	sess := &loginSession{
		cmd:       cmd,
		ptmx:      ptmx,
		startedAt: time.Now(),
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	s.session = sess
	s.setStateLocked(LoginState{Active: true, StartedAt: sess.startedAt.Unix()})
	s.mu.Unlock()

	urlFound := make(chan string, 1)
	go readLoginOutput(sess, urlFound)
	go waitForLoginExit(sess)

	select {
	case url := <-urlFound:
		s.setState(LoginState{Active: true, AuthURL: url, AwaitingCode: true, StartedAt: sess.startedAt.Unix()})
		return StartResult{URL: url}, nil
	case <-time.After(claudeURLReadWait):
		cancel()
		s.clear(sess)
		err := fmt.Errorf("did not see Anthropic OAuth URL within %s; first 500 bytes of claude output: %s",
			claudeURLReadWait, truncate(sess.Output(), 500))
		s.setState(LoginState{Error: err.Error()})
		return StartResult{}, err
	case <-sess.done:
		s.clear(sess)
		msg := "claude exited before printing OAuth URL"
		if exitErr := sess.ExitErr(); exitErr != nil {
			msg += " (" + exitErr.Error() + ")"
		}
		err := fmt.Errorf("%s; output: %s", msg, truncate(sess.Output(), 500))
		s.setState(LoginState{Error: err.Error()})
		return StartResult{}, err
	case <-ctx.Done():
		cancel()
		s.clear(sess)
		s.setState(LoginState{Error: ctx.Err().Error()})
		return StartResult{}, ctx.Err()
	}
}

func (s *Service) SubmitCode(ctx context.Context, code string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrCodeRequired
	}

	sess := s.current()
	if sess == nil {
		return ErrNoSession
	}

	if _, err := io.WriteString(sess.ptmx, code+"\r"); err != nil {
		err = fmt.Errorf("write to claude stdin: %w", err)
		s.setState(LoginState{Error: err.Error()})
		return err
	}

	select {
	case <-sess.done:
	case <-time.After(claudeExitWait):
		sess.cancel()
		<-sess.done
		s.clear(sess)
		err := fmt.Errorf("claude did not exit within %s after code paste; last output: %s",
			claudeExitWait, truncate(sess.Output(), 500))
		s.setState(LoginState{Error: err.Error()})
		return err
	case <-ctx.Done():
		return ctx.Err()
	}

	debugOut := sess.Output()
	exitErr := sess.ExitErr()
	s.clear(sess)

	if exitErr != nil && !errors.Is(exitErr, context.Canceled) {
		err := fmt.Errorf("claude exited with error: %w; output: %s", exitErr, truncate(debugOut, 500))
		s.setState(LoginState{Error: err.Error()})
		return err
	}
	if !authenticated() {
		err := fmt.Errorf("claude exited cleanly but no credentials file was written; output: %s",
			truncate(debugOut, 500))
		s.setState(LoginState{Error: err.Error()})
		return err
	}
	s.setState(LoginState{Completed: true})
	return nil
}

func (s *Service) Cancel(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sess := s.current()
	if sess == nil {
		return nil
	}
	sess.cancel()

	select {
	case <-sess.done:
		s.clear(sess)
		s.setState(LoginState{})
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) current() *loginSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

func (s *Service) clear(sess *loginSession) {
	s.mu.Lock()
	if s.session == sess {
		s.session = nil
	}
	s.mu.Unlock()
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
