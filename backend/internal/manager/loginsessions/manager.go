package loginsessions

// Package loginsessions owns the lifecycle of headed Chromium sessions used
// to capture login state (cookies + localStorage) for an arbitrary external
// site, on behalf of a project member. Each session runs inside the
// project's LXD container with --remote-debugging-port exposed on the
// container interface so the host backend can drive it over CDP.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	minPort        = 19000
	maxPort        = 19999
	maxAttempts    = 32
	startTimeout   = 90 * time.Second
	stopTimeout    = 30 * time.Second
	probeTimeout   = 3 * time.Second
	expireAfter    = 15 * time.Minute
)

// LXCRunner runs `lxc <args>` and returns combined stdout+stderr. The
// loginsessions manager uses this exclusively to talk to the host's LXD
// CLI; it never invokes lxc directly.
type LXCRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// ProjectMeta is the subset of project.Meta we keep on each session — slug
// is what resolves to the container hostname (<slug>.lxd), ContainerName
// is the lxc target name (currently == slug, but kept distinct for clarity).
type ProjectMeta struct {
	ID            string
	Slug          string
	ContainerName string
}

// Session is one in-flight Chromium login session.
type Session struct {
	ID            string      `json:"id"`
	ProjectID     string      `json:"projectId"`
	ContainerName string      `json:"containerName"`
	Slug          string      `json:"slug"`
	URL           string      `json:"url"`
	Name          string      `json:"name"`
	SecretName    string      `json:"secretName"`
	Port          int         `json:"port"`
	UserDataDir   string      `json:"userDataDir"`
	StartedAt     time.Time   `json:"startedAt"`
	ExpiresAt     time.Time   `json:"expiresAt"`
}

// Manager owns the in-memory map of sessions.
type Manager struct {
	runner LXCRunner

	mu       sync.Mutex
	sessions map[string]*Session

	// portsInUse tracks which container ports we've allocated to avoid races.
	portsInUse map[string]map[int]struct{}
}

// New returns a manager wired up to the supplied lxc runner. The caller is
// expected to keep the same instance for the lifetime of the process.
func New(runner LXCRunner) *Manager {
	return &Manager{
		runner:     runner,
		sessions:   map[string]*Session{},
		portsInUse: map[string]map[int]struct{}{},
	}
}

// SanitizeSecretName turns an arbitrary user-supplied "name" into a valid
// project secret key of the form STORAGE_STATE_<UPPER>. Returns the full
// secret key plus the cleaned suffix; the empty string + error if nothing
// usable survives the sanitize.
func SanitizeSecretName(name string) (string, string, error) {
	clean := strings.Builder{}
	for _, r := range strings.ToUpper(strings.TrimSpace(name)) {
		switch {
		case r >= 'A' && r <= 'Z':
			clean.WriteRune(r)
		case r >= '0' && r <= '9':
			clean.WriteRune(r)
		case r == '_' || r == '-' || r == ' ':
			clean.WriteRune('_')
		}
	}
	suffix := strings.Trim(clean.String(), "_")
	for strings.Contains(suffix, "__") {
		suffix = strings.ReplaceAll(suffix, "__", "_")
	}
	if suffix == "" {
		return "", "", errors.New("name must contain at least one letter or digit")
	}
	if suffix[0] >= '0' && suffix[0] <= '9' {
		suffix = "X" + suffix
	}
	return "STORAGE_STATE_" + suffix, suffix, nil
}

// Start allocates a Chromium login session inside the given project's
// container. Returns the session record on success; on error the partial
// state is cleaned up before returning.
func (m *Manager) Start(ctx context.Context, project ProjectMeta, url, name string) (*Session, error) {
	if project.ContainerName == "" {
		return nil, errors.New("project has no container")
	}
	if strings.TrimSpace(url) == "" {
		return nil, errors.New("url is required")
	}
	secretName, _, err := SanitizeSecretName(name)
	if err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	id := randomID(8)
	port, err := m.allocatePort(project.ContainerName)
	if err != nil {
		return nil, err
	}
	userDataDir := fmt.Sprintf("/tmp/login-%s", id)

	// Build a one-liner that:
	//  1. Lazy-installs Chromium via Playwright's bundled binary if needed.
	//  2. Apt-installs Chromium's system deps (best-effort - quiet on success).
	//  3. Spawns Chromium with --remote-debugging-* and goes to the URL.
	script := buildLaunchScript(launchScriptArgs{
		Port:        port,
		UserDataDir: userDataDir,
		URL:         url,
	})

	if _, err := m.runner.Run(ctx, "exec", project.ContainerName, "--", "bash", "-lc", script); err != nil {
		m.releasePort(project.ContainerName, port)
		return nil, fmt.Errorf("launch chromium: %w", err)
	}

	// Wait until DevTools endpoint is up.
	target := fmt.Sprintf("%s.lxd:%d", project.Slug, port)
	if err := waitForDevTools(ctx, target, startTimeout); err != nil {
		// best-effort cleanup
		killCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		_, _ = m.runner.Run(killCtx, "exec", project.ContainerName, "--", "bash", "-lc",
			fmt.Sprintf("pkill -f 'remote-debugging-port=%d' || true; rm -rf %s", port, shellQuote(userDataDir)))
		m.releasePort(project.ContainerName, port)
		return nil, fmt.Errorf("chromium devtools not reachable at %s: %w", target, err)
	}

	now := time.Now()
	sess := &Session{
		ID:            id,
		ProjectID:     project.ID,
		ContainerName: project.ContainerName,
		Slug:          project.Slug,
		URL:           url,
		Name:          name,
		SecretName:    secretName,
		Port:          port,
		UserDataDir:   userDataDir,
		StartedAt:     now,
		ExpiresAt:     now.Add(expireAfter),
	}

	m.mu.Lock()
	m.sessions[id] = sess
	m.mu.Unlock()

	// Auto-expire timer.
	go m.expireAfter(sess)

	return sess, nil
}

// Get returns a copy of the session by id.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[id]
	if !ok {
		return nil, false
	}
	cp := *sess
	return &cp, true
}

// Stop terminates a session and cleans up. It is safe to call multiple
// times — subsequent calls return nil.
func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	m.releasePort(sess.ContainerName, sess.Port)
	script := fmt.Sprintf("pkill -f 'remote-debugging-port=%d' || true; rm -rf %s",
		sess.Port, shellQuote(sess.UserDataDir))
	_, err := m.runner.Run(ctx, "exec", sess.ContainerName, "--", "bash", "-lc", script)
	return err
}

// DevToolsAddr returns the host:port the host backend can reach the
// session's DevTools endpoint on.
func (s *Session) DevToolsAddr() string {
	return fmt.Sprintf("%s.lxd:%d", s.Slug, s.Port)
}

func (m *Manager) allocatePort(container string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.portsInUse[container]; !ok {
		m.portsInUse[container] = map[int]struct{}{}
	}
	for i := 0; i < maxAttempts; i++ {
		buf := make([]byte, 2)
		if _, err := rand.Read(buf); err != nil {
			return 0, err
		}
		port := minPort + int(uint16(buf[0])<<8|uint16(buf[1]))%(maxPort-minPort+1)
		if _, taken := m.portsInUse[container][port]; taken {
			continue
		}
		m.portsInUse[container][port] = struct{}{}
		return port, nil
	}
	return 0, errors.New("could not allocate a free login-session port")
}

func (m *Manager) releasePort(container string, port int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if set, ok := m.portsInUse[container]; ok {
		delete(set, port)
		if len(set) == 0 {
			delete(m.portsInUse, container)
		}
	}
}

func (m *Manager) expireAfter(sess *Session) {
	timer := time.NewTimer(time.Until(sess.ExpiresAt))
	defer timer.Stop()
	<-timer.C
	// Only stop if it's still the same session (i.e. not stopped already).
	m.mu.Lock()
	if cur, ok := m.sessions[sess.ID]; !ok || cur != sess {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	_ = m.Stop(ctx, sess.ID)
}

// waitForDevTools polls <target>/json/version until it returns 200 or the
// deadline expires.
func waitForDevTools(ctx context.Context, target string, total time.Duration) error {
	deadline := time.Now().Add(total)
	client := &http.Client{Timeout: probeTimeout}
	url := "http://" + target + "/json/version"

	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(750 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("timeout")
	}
	return lastErr
}

// FetchVersion fetches the /json/version response (used to discover the
// browser WS endpoint).
func FetchVersion(ctx context.Context, target string) (map[string]any, error) {
	client := &http.Client{Timeout: probeTimeout}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+target+"/json/version", nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	out := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func randomID(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// Fall back to time-based — should never happen but better than panic.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// shellQuote produces a single-quoted shell literal that's safe to splice
// into a bash -c argument. No interpolation.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

