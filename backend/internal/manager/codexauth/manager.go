package codexauth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const loginTimeout = 30 * time.Second

var (
	ErrAPIKeyRequired = errors.New("OpenAI API key is required")
	ErrCodexNotFound  = errors.New("codex CLI not found on PATH - install it first")
)

type Manager struct{}

func New() *Manager {
	return &Manager{}
}

func (m *Manager) Authenticated() bool {
	return authenticated()
}

func (m *Manager) LoginWithAPIKey(ctx context.Context, apiKey string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ErrAPIKeyRequired
	}
	if _, err := exec.LookPath("codex"); err != nil {
		return ErrCodexNotFound
	}

	loginCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	cmd := exec.CommandContext(loginCtx, "codex", "login", "--with-api-key")
	cmd.Env = codexEnv(os.Environ())
	cmd.Stdin = strings.NewReader(apiKey + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codex login: %w; output: %s", err, truncate(string(out), 500))
	}
	if !authenticated() {
		return fmt.Errorf("codex login completed but no auth file was written; output: %s",
			truncate(string(out), 500))
	}
	return nil
}

func authenticated() bool {
	if _, err := os.Stat(filepath.Join(codexHomeDir(), "auth.json")); err == nil {
		return true
	}
	return false
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
	for _, env := range base {
		if strings.HasPrefix(env, "CODEX_HOME=") {
			return base
		}
	}
	return append(base, "CODEX_HOME="+codexHomeDir())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
