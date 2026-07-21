package containers

// Kimi-specific provider glue: installing @moonshot-ai/kimi-code inside a
// container and shipping the Kimi Code OAuth credential directory into each
// project rootfs. Unlike Claude/Codex (a single credential file), kimi-code
// stores rotating OAuth tokens under ~/.kimi-code/credentials/, so we
// push/pull the whole directory.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	hostKimiCredsDir      = "/root/.kimi-code/credentials"
	containerKimiHomeDir  = "/root/.kimi-code"
	containerKimiCredsDir = "/root/.kimi-code/credentials"
)

// EnsureKimi installs the @moonshot-ai/kimi-code CLI inside the container if it
// isn't already present. Mirrors EnsureClaude: the shared BaseImageInstallScript
// (Node 22 + `npm i -g` the agent CLIs) is the install path, so older Node-20
// containers self-heal on first use.
func (m *Manager) EnsureKimi(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	if m.kimiInstalled(ctx, containerName) {
		return nil
	}
	if m.kimiInstallRunning(ctx, containerName) {
		waitCtx, cancelW := context.WithTimeout(ctx, 5*time.Minute)
		defer cancelW()
		if err := m.waitForKimi(waitCtx, containerName); err == nil {
			return nil
		}
	}

	installCtx, cancelI := context.WithTimeout(ctx, 8*time.Minute)
	defer cancelI()
	out, err := m.lxc.Run(installCtx, "exec", containerName, "--", "bash", "-c", BaseImageInstallScript)
	if err != nil {
		waitCtx, cancelW := context.WithTimeout(ctx, 90*time.Second)
		defer cancelW()
		if waitErr := m.waitForKimi(waitCtx, containerName); waitErr == nil {
			return nil
		}
		return fmt.Errorf("install kimi in %s: %w; output: %s",
			containerName, err, truncateOut(out, 1000))
	}
	return nil
}

func (m *Manager) kimiInstalled(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := m.lxc.Run(quickCtx, "exec", containerName, "--", "which", "kimi")
	return err == nil
}

func (m *Manager) kimiInstallRunning(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := m.lxc.Run(quickCtx, "exec", containerName, "--",
		"pgrep", "-f", "npm install.*@moonshot-ai/kimi-code")
	return err == nil && strings.TrimSpace(out) != ""
}

func (m *Manager) waitForKimi(ctx context.Context, containerName string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if m.kimiInstalled(ctx, containerName) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// EnsureKimiAuth pushes the host's Kimi Code OAuth credentials into the
// container. Two supported modes:
//   - host-canonical: the host ran `kimi login`; we push each credential file
//     when the host copy is newer (and sync rotations back after the run).
//   - per-container: the host has no credentials, but the container itself was
//     logged in (`kimi login` inside it). We leave its credentials untouched.
//
// Errors only when neither the host nor the container is authenticated.
func (m *Manager) EnsureKimiAuth(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	files, err := hostKimiCredFiles()
	if err != nil || len(files) == 0 {
		if m.kimiContainerAuthed(ctx, containerName) {
			return nil
		}
		return fmt.Errorf("kimi not authenticated — run `kimi login` on the host or in container %s", containerName)
	}

	pctx, cancel := context.WithTimeout(ctx, authPushTimeout)
	defer cancel()

	for _, dir := range []string{containerKimiHomeDir, containerKimiCredsDir} {
		if out, err := m.lxc.Run(pctx, "exec", containerName, "--", "install", "-d", "-m", "700", dir); err != nil {
			return fmt.Errorf("mkdir %s in container: %w; output: %s", dir, err, out)
		}
	}

	for _, name := range files {
		host := filepath.Join(hostKimiCredsDir, name)
		container := containerKimiCredsDir + "/" + name
		if err := m.pushKimiFileIfNewer(pctx, host, container, containerName); err != nil {
			return fmt.Errorf("push %s: %w", container, err)
		}
	}
	return nil
}

// SyncKimiAuthFromContainer pulls rotated Kimi credentials back to the host
// after a run (kimi-code rotates its OAuth refresh token). Only meaningful in
// host-canonical mode; in per-container mode (no host credentials) it is a
// no-op so one container's identity is never copied onto the host.
func (m *Manager) SyncKimiAuthFromContainer(ctx context.Context, containerName string) error {
	if !m.Available() {
		return nil
	}
	if files, err := hostKimiCredFiles(); err != nil || len(files) == 0 {
		return nil
	}

	pctx, cancel := context.WithTimeout(ctx, authPushTimeout)
	defer cancel()

	out, err := m.lxc.Run(pctx, "exec", containerName, "--",
		"find", containerKimiCredsDir, "-maxdepth", "1", "-type", "f", "-printf", "%f\n")
	if err != nil {
		return nil
	}
	for _, name := range strings.Fields(out) {
		container := containerKimiCredsDir + "/" + name
		host := filepath.Join(hostKimiCredsDir, name)
		if out, err := m.lxc.Run(pctx, "file", "pull", containerName+container, host); err != nil {
			return fmt.Errorf("pull %s: %w; output: %s", container, err, out)
		}
		_ = os.Chmod(host, 0o600)
		now := time.Now()
		_ = os.Chtimes(host, now, now)
	}
	return nil
}

func (m *Manager) kimiContainerAuthed(ctx context.Context, containerName string) bool {
	quickCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := m.lxc.Run(quickCtx, "exec", containerName, "--",
		"sh", "-c", "ls -1 "+containerKimiCredsDir+" 2>/dev/null | head -1")
	return err == nil && strings.TrimSpace(out) != ""
}

func hostKimiCredFiles() ([]string, error) {
	entries, err := os.ReadDir(hostKimiCredsDir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

func (m *Manager) pushKimiFileIfNewer(ctx context.Context, hostPath, containerPath, containerName string) error {
	hostInfo, err := os.Stat(hostPath)
	if err != nil {
		return err
	}
	shouldPush := true
	if out, err := m.lxc.Run(ctx, "exec", containerName, "--", "stat", "-c", "%Y", containerPath); err == nil {
		if containerUnix, perr := strconv.ParseInt(strings.TrimSpace(out), 10, 64); perr == nil {
			shouldPush = hostInfo.ModTime().Unix() > containerUnix
		}
	}
	if !shouldPush {
		return nil
	}
	if out, err := m.lxc.Run(ctx, "file", "push", "--mode=600", hostPath, containerName+containerPath); err != nil {
		return fmt.Errorf("lxc file push: %w; output: %s", err, out)
	}
	return nil
}
