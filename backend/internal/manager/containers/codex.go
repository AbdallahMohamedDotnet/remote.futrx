package containers

// Codex-specific provider glue: requiring the OpenAI Codex CLI inside a
// container and shipping the Codex auth file into each project rootfs.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	ErrCodexMissing    = errors.New("Codex CLI is missing from the project container; recreate the container from futrx-remote-dev-base")
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
	if strings.EqualFold(strings.TrimSpace(mode), "apikey") {
		return true
	}
	_, ok := raw["OPENAI_API_KEY"]
	return ok
}

func (m *Manager) EnsureCodex(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	if m.codexInstalled(ctx, containerName) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrCodexMissing, containerName)
}

func (m *Manager) codexInstalled(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := m.lxc.Run(quickCtx, "exec", containerName, "--", "which", "codex")
	return err == nil
}
