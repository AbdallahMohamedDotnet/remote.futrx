package codexauth

import (
	"bufio"
	"context"
	"encoding/json"
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
	deviceLoginTimeout      = 16 * time.Minute
	deviceLoginTTL          = 15 * time.Minute
)

var (
	ErrCodexNotFound = errors.New("codex CLI not found on PATH - install it first")

	ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	deviceURLRE  = regexp.MustCompile(`https://auth\.openai\.com/codex/device`)
	deviceCodeRE = regexp.MustCompile(`[A-Z0-9]{4}-[A-Z0-9]{5}`)
)

type Status struct {
	Authenticated bool             `json:"authenticated"`
	AuthMode      string           `json:"authMode,omitempty"`
	UsesAPIKey    bool             `json:"usesApiKey,omitempty"`
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
	authenticated, _, _ := authenticated()
	return authenticated
}

func (m *Manager) Status() Status {
	authenticated, authMode, usesAPIKey := authenticated()
	return Status{
		Authenticated: authenticated,
		AuthMode:      authMode,
		UsesAPIKey:    usesAPIKey,
		DeviceLogin:   m.deviceSnapshot(),
	}
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
	if _, err := exec.LookPath("codex"); err != nil {
		return DeviceLoginState{}, ErrCodexNotFound
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
	cmd := exec.CommandContext(loginCtx, "codex", "login", "--device-auth")
	cmd.Env = codexEnv(os.Environ())

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
		m.device = DeviceLoginState{Error: fmt.Sprintf("start codex login: %v", err)}
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
	authenticated, _, usesAPIKey := authenticated()
	switch {
	case authenticated:
		state.Completed = true
		state.Error = ""
	case usesAPIKey:
		state.Error = "Codex is still logged in with an API key. Sign in with ChatGPT to use subscription limits."
	case err != nil:
		state.Error = fmt.Sprintf("codex login failed: %s", truncate(err.Error(), 300))
	default:
		state.Error = "Codex login ended before authentication completed."
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
	authenticated, authMode, usesAPIKey := authenticated()
	return Status{
		Authenticated: authenticated,
		AuthMode:      authMode,
		UsesAPIKey:    usesAPIKey,
		DeviceLogin:   m.device,
	}
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

func authenticated() (bool, string, bool) {
	authPath := filepath.Join(codexHomeDir(), "auth.json")
	authMode, usesAPIKey := codexAuthMode(authPath)
	if usesAPIKey {
		return false, authMode, true
	}
	if authMode == "" {
		return false, "", false
	}
	return true, authMode, false
}

func codexAuthMode(authPath string) (string, bool) {
	data, err := os.ReadFile(authPath)
	if err != nil {
		return "", false
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "unknown", false
	}
	mode, _ := raw["auth_mode"].(string)
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		if _, hasAPIKey := raw["OPENAI_API_KEY"]; hasAPIKey {
			return "apikey", true
		}
		return "unknown", false
	}
	return mode, mode == "apikey"
}

func codexHomeDir() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	if v := os.Getenv("HOME"); v != "" {
		return filepath.Join(v, ".codex")
	}
	return "/root/.codex"
}

func codexEnv(base []string) []string {
	out := make([]string, 0, len(base)+1)
	hasCodexHome := false
	for _, env := range base {
		if strings.HasPrefix(env, "OPENAI_API_KEY=") {
			continue
		}
		if strings.HasPrefix(env, "CODEX_HOME=") {
			hasCodexHome = true
		}
		out = append(out, env)
	}
	if hasCodexHome {
		return out
	}
	return append(out, "CODEX_HOME="+codexHomeDir())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
