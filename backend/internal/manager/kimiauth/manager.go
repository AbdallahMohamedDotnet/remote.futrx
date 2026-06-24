package kimiauth

// Manager drives the host-side `kimi login` device-code flow for
// @moonshot-ai/kimi-code and streams its state to subscribers. Unlike Codex
// there is no API-key mode: Kimi Code auth is always a subscription OAuth
// grant, stored under ~/.kimi-code/credentials/.

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
)

const (
	deviceLoginReadyTimeout = 8 * time.Second
	deviceLoginTimeout      = 30 * time.Minute
	deviceLoginTTL          = 29 * time.Minute
)

var (
	ErrKimiNotFound = errors.New("kimi CLI not found on PATH - install it first")

	ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// kimi prints e.g.
	//   https://www.kimi.com/code/authorize_device?user_code=T906-Q0QV
	deviceURLRE  = regexp.MustCompile(`https://www\.kimi\.com/code/authorize_device\S*`)
	deviceCodeRE = regexp.MustCompile(`[A-Z0-9]{4}-[A-Z0-9]{4,6}`)
)

type Status struct {
	Authenticated bool             `json:"authenticated"`
	DeviceLogin   DeviceLoginState `json:"deviceLogin,omitempty"`
}

type DeviceLoginState struct {
	Active          bool   `json:"active"`
	VerificationURI string `json:"verificationUri,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
	StartedAt       int64  `json:"startedAt,omitempty"`
	ExpiresAt       int64  `json:"expiresAt,omitempty"`
	Completed       bool   `json:"completed,omitempty"`
	Error           string `json:"error,omitempty"`
}

type Manager struct {
	mu     sync.Mutex
	device DeviceLoginState
	cancel context.CancelFunc
	subs   map[chan Status]struct{}
}

func New() *Manager {
	return &Manager{subs: map[chan Status]struct{}{}}
}

func (m *Manager) Authenticated() bool {
	return authenticated()
}

func (m *Manager) Status() Status {
	return Status{Authenticated: authenticated(), DeviceLogin: m.deviceSnapshot()}
}

func (m *Manager) Subscribe() (<-chan Status, func()) {
	ch := make(chan Status, 8)
	m.mu.Lock()
	if m.subs == nil {
		m.subs = map[chan Status]struct{}{}
	}
	m.subs[ch] = struct{}{}
	status := m.statusLocked()
	m.mu.Unlock()
	ch <- status

	cancel := func() {
		m.mu.Lock()
		if _, ok := m.subs[ch]; ok {
			delete(m.subs, ch)
			close(ch)
		}
		m.mu.Unlock()
	}
	return ch, cancel
}

func (m *Manager) StartDeviceLogin(ctx context.Context) (DeviceLoginState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := exec.LookPath("kimi"); err != nil {
		return DeviceLoginState{}, ErrKimiNotFound
	}

	now := time.Now()

	m.mu.Lock()
	if m.device.Active && now.Unix() < m.device.ExpiresAt {
		state := m.device
		m.mu.Unlock()
		return state, nil
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	loginCtx, cancel := context.WithTimeout(context.Background(), deviceLoginTimeout)
	cmd := exec.CommandContext(loginCtx, "kimi", "login")
	cmd.Env = kimiEnv(os.Environ())

	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer

	state := DeviceLoginState{
		Active:    true,
		StartedAt: now.Unix(),
		ExpiresAt: now.Add(deviceLoginTTL).Unix(),
	}
	m.device = state
	m.cancel = cancel

	ready := make(chan struct{})
	var readyOnce sync.Once
	markReady := func() {
		readyOnce.Do(func() {
			close(ready)
		})
	}

	if err := cmd.Start(); err != nil {
		cancel()
		_ = reader.Close()
		_ = writer.Close()
		m.device = DeviceLoginState{Error: fmt.Sprintf("start kimi login: %v", err)}
		m.cancel = nil
		state = m.device
		m.broadcastLocked()
		m.mu.Unlock()
		return state, err
	}

	go m.consumeDeviceLoginOutput(reader, markReady)
	done := make(chan struct{})
	go func() {
		err := cmd.Wait()
		_ = writer.Close()
		m.finishDeviceLogin(err)
		close(done)
	}()

	m.mu.Unlock()
	m.Broadcast()

	select {
	case <-ready:
		return m.deviceSnapshot(), nil
	case <-done:
		return m.deviceSnapshot(), nil
	case <-time.After(deviceLoginReadyTimeout):
		return m.deviceSnapshot(), nil
	case <-ctx.Done():
		return m.deviceSnapshot(), ctx.Err()
	}
}

func (m *Manager) deviceSnapshot() DeviceLoginState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.device
}

func (m *Manager) consumeDeviceLoginOutput(reader io.Reader, markReady func()) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := ansiEscapeRE.ReplaceAllString(scanner.Text(), "")
		changed := false
		m.mu.Lock()
		if url := deviceURLRE.FindString(line); url != "" {
			changed = changed || m.device.VerificationURI != url
			m.device.VerificationURI = url
		}
		if code := deviceCodeRE.FindString(line); code != "" {
			changed = changed || m.device.UserCode != code
			m.device.UserCode = code
			if m.device.ExpiresAt == 0 {
				m.device.ExpiresAt = time.Now().Add(deviceLoginTTL).Unix()
			}
		}
		ready := m.device.VerificationURI != "" && m.device.UserCode != ""
		if changed {
			m.broadcastLocked()
		}
		m.mu.Unlock()
		if ready {
			markReady()
		}
	}
}

func (m *Manager) finishDeviceLogin(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	state := m.device
	state.Active = false
	switch {
	case authenticated():
		state.Completed = true
		state.Error = ""
	case err != nil:
		state.Error = fmt.Sprintf("kimi login failed: %s", truncate(err.Error(), 300))
	default:
		state.Error = "Kimi login ended before authentication completed."
	}
	m.device = state
	m.broadcastLocked()
}

func (m *Manager) Broadcast() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastLocked()
}

func (m *Manager) statusLocked() Status {
	return Status{Authenticated: authenticated(), DeviceLogin: m.device}
}

func (m *Manager) broadcastLocked() {
	status := m.statusLocked()
	for ch := range m.subs {
		select {
		case ch <- status:
		default:
			delete(m.subs, ch)
			close(ch)
		}
	}
}

// authenticated reports whether a Kimi Code OAuth credential exists on the
// host (any regular file under ~/.kimi-code/credentials/).
func authenticated() bool {
	entries, err := os.ReadDir(kimiCredsDir())
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			return true
		}
	}
	return false
}

func kimiHomeDir() string {
	if v := os.Getenv("KIMI_CODE_HOME"); v != "" {
		return v
	}
	if v := os.Getenv("HOME"); v != "" {
		return filepath.Join(v, ".kimi-code")
	}
	return "/root/.kimi-code"
}

func kimiCredsDir() string {
	return filepath.Join(kimiHomeDir(), "credentials")
}

func kimiEnv(base []string) []string {
	for _, env := range base {
		if strings.HasPrefix(env, "KIMI_CODE_HOME=") {
			return base
		}
	}
	return append(base, "KIMI_CODE_HOME="+kimiHomeDir())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
