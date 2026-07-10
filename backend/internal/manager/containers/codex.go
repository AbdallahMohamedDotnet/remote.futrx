package containers

// Codex-specific provider glue: requiring the OpenAI Codex CLI inside a
// container and shipping the Codex auth file into each project rootfs.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

const (
	hostCodexDir  = "/root/.codex"
	hostCodexAuth = "/root/.codex/auth.json"

	containerCodexDir  = "/root/.codex"
	containerCodexAuth = "/root/.codex/auth.json"
)

var (
	ErrCodexAPIKeyAuth = errors.New("Codex is logged in with an API key; run codex login with ChatGPT to use subscription limits")
)

func CodexAuthBundle() AuthBundle {
	return AuthBundle{
		Name:         "codex",
		HostDir:      hostCodexDir,
		ContainerDir: containerCodexDir,
		Files: []AuthFile{
			{
				HostPath:      hostCodexAuth,
				ContainerPath: containerCodexAuth,
				Mode:          "600",
				PushRequired:  true,
				PullRequired:  true,
			},
		},
	}
}

func (m *Manager) EnsureCodexAuth(ctx context.Context, containerName string) error {
	if codexAuthUsesAPIKey(hostCodexAuth) {
		return ErrCodexAPIKeyAuth
	}
	return m.EnsureAuthBundle(ctx, containerName, CodexAuthBundle())
}

func (m *Manager) SyncCodexAuthFromContainer(ctx context.Context, containerName string) error {
	if err := m.SyncAuthBundleFromContainer(ctx, containerName, CodexAuthBundle()); err != nil {
		return err
	}
	if codexAuthUsesAPIKey(hostCodexAuth) {
		return ErrCodexAPIKeyAuth
	}
	return nil
}

func codexAuthUsesAPIKey(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	mode, _ := raw["auth_mode"].(string)
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		_, ok := raw["OPENAI_API_KEY"]
		return ok
	}
	return mode == "apikey"
}

// EnsureCodex installs or upgrades Codex to the repository pin. This keeps
// existing project containers compatible with newly exposed models such as
// GPT-5.6 Sol without requiring a destructive container recreation.
func (m *Manager) EnsureCodex(ctx context.Context, containerName string) error {
	return m.ensureAgentCLI(ctx, containerName, codexCLISpec)
}
