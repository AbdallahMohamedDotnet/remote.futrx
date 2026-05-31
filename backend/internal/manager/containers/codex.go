package containers

// Codex-specific provider glue: installing the OpenAI Codex CLI inside a
// container and shipping the Codex auth file into each project rootfs.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	hostCodexDir  = "/root/.codex"
	hostCodexAuth = "/root/.codex/auth.json"

	containerCodexDir  = "/root/.codex"
	containerCodexAuth = "/root/.codex/auth.json"
)

var ErrCodexAPIKeyAuth = errors.New("Codex is logged in with an API key; run codex login with ChatGPT to use subscription limits")

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
	if m.codexInstallRunning(ctx, containerName) {
		waitCtx, cancelW := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelW()
		if err := m.waitForCodex(waitCtx, containerName); err == nil {
			return nil
		}
	}

	installCtx, cancelI := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelI()
	out, err := m.lxc.Run(installCtx, "exec", containerName, "--", "bash", "-c", BaseImageInstallScript)
	if err != nil {
		waitCtx, cancelW := context.WithTimeout(ctx, 90*time.Second)
		defer cancelW()
		if waitErr := m.waitForCodex(waitCtx, containerName); waitErr == nil {
			return nil
		}
		return fmt.Errorf("install codex in %s: %w; output: %s",
			containerName, err, truncateOut(out, 1000))
	}
	return nil
}

func (m *Manager) codexInstalled(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := m.lxc.Run(quickCtx, "exec", containerName, "--", "which", "codex")
	return err == nil
}

func (m *Manager) codexInstallRunning(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := m.lxc.Run(quickCtx, "exec", containerName, "--",
		"pgrep", "-f", "npm install.*@openai/codex")
	return err == nil && strings.TrimSpace(out) != ""
}

func (m *Manager) waitForCodex(ctx context.Context, containerName string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if m.codexInstalled(ctx, containerName) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
