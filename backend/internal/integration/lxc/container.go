package lxc

// Wrapper around the `lxc` CLI to launch / inspect / delete project containers.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const (
	hostMappedUID = 1000000

	defaultImage = "ubuntu:24.04"
	containerWS  = "/workspace"

	hostClaudeJSON  = "/root/.claude.json"
	hostClaudeCreds = "/root/.claude/.credentials.json"

	launchTimeout = 90 * time.Second
	startTimeout  = 30 * time.Second
	stopTimeout   = 30 * time.Second
	deleteTimeout = 30 * time.Second
	queryTimeout  = 10 * time.Second
)

type Manager struct {
	image string
}

func New() *Manager {
	return &Manager{image: defaultImage}
}

func (m *Manager) Available() bool {
	_, err := exec.LookPath("lxc")
	return err == nil
}

func (m *Manager) Launch(ctx context.Context, p serviceproject.Meta) error {
	if !m.Available() {
		return errors.New("lxc CLI not found on PATH - install LXD on the host first")
	}

	if err := os.MkdirAll(p.Cwd, 0o755); err != nil {
		return fmt.Errorf("create workspace dir: %w", err)
	}
	if err := chownRecursive(p.Cwd, hostMappedUID, hostMappedUID); err != nil {
		return fmt.Errorf("chown workspace: %w", err)
	}

	state, err := m.State(ctx, p.ContainerName)
	if err != nil {
		return err
	}
	if state != serviceproject.ContainerStateMissing {
		if state == serviceproject.ContainerStateStopped {
			return m.Start(ctx, p.ContainerName)
		}
		return nil
	}

	lctx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()
	if out, err := lxcRun(lctx, "launch", m.image, p.ContainerName); err != nil {
		return fmt.Errorf("lxc launch: %w; output: %s", err, out)
	}

	if err := m.attachDisk(ctx, p.ContainerName, "workspace", p.Cwd, containerWS, false); err != nil {
		return fmt.Errorf("attach workspace: %w", err)
	}

	if err := m.EnsureBootAutostart(ctx, p.ContainerName); err != nil {
		_ = err
	}

	if err := m.EnsureClaudeAuth(ctx, p.ContainerName); err != nil {
		_ = err
	}

	return nil
}

func (m *Manager) attachDisk(ctx context.Context, container, deviceName, hostSrc, containerPath string, readonly bool) error {
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	args := []string{
		"config", "device", "add", container, deviceName, "disk",
		"source=" + hostSrc,
		"path=" + containerPath,
	}
	if readonly {
		args = append(args, "readonly=true")
	}
	if out, err := lxcRun(lctx, args...); err != nil {
		return fmt.Errorf("lxc config device add %s: %w; output: %s", deviceName, err, out)
	}
	return nil
}

// EnsureBootAutostart marks the container as auto-starting after a host
// reboot. Without this, a host reboot leaves every project's container in
// `stopped` state until the runner brings it back via a prompt — which
// silently breaks anything time-driven (cron, systemd timers). Idempotent;
// cheap; safe to call on every prompt as a migration for older containers.
func (m *Manager) EnsureBootAutostart(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	// `lxc config get` returns empty (no error) when unset; "true" once set.
	cur, _ := lxcRun(lctx, "config", "get", containerName, "boot.autostart")
	if strings.TrimSpace(cur) == "true" {
		return nil
	}
	if out, err := lxcRun(lctx, "config", "set", containerName, "boot.autostart", "true"); err != nil {
		return fmt.Errorf("set boot.autostart: %w; output: %s", err, out)
	}
	return nil
}

func (m *Manager) Start(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	if out, err := lxcRun(lctx, "start", containerName); err != nil {
		if strings.Contains(out, "is already running") {
			return nil
		}
		return fmt.Errorf("lxc start: %w; output: %s", err, out)
	}
	return nil
}

func (m *Manager) EnsureClaudeAuth(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}

	dctx, cancelD := context.WithTimeout(ctx, queryTimeout)
	_, _ = lxcRun(dctx, "config", "device", "remove", containerName, "claude-auth")
	cancelD()

	qctx, cancelQ := context.WithTimeout(ctx, queryTimeout)
	defer cancelQ()
	if _, err := lxcRun(qctx, "exec", containerName, "--",
		"test", "-f", "/root/.claude/.credentials.json"); err == nil {
		return nil
	}

	if _, err := os.Stat(hostClaudeJSON); err != nil {
		return fmt.Errorf("host claude not authenticated yet: %s missing", hostClaudeJSON)
	}

	pctx, cancelP := context.WithTimeout(ctx, 30*time.Second)
	defer cancelP()

	if out, err := lxcRun(pctx, "exec", containerName, "--",
		"install", "-d", "-m", "700", "/root/.claude"); err != nil {
		return fmt.Errorf("mkdir /root/.claude in container: %w; output: %s", err, out)
	}

	if out, err := lxcRun(pctx, "file", "push", "--mode=600",
		hostClaudeJSON, containerName+"/root/.claude.json"); err != nil {
		return fmt.Errorf("push .claude.json: %w; output: %s", err, out)
	}

	if _, err := os.Stat(hostClaudeCreds); err == nil {
		if out, err := lxcRun(pctx, "file", "push", "--mode=600",
			hostClaudeCreds, containerName+"/root/.claude/.credentials.json"); err != nil {
			return fmt.Errorf("push .credentials.json: %w; output: %s", err, out)
		}
	}

	return nil
}

func (m *Manager) EnsureClaude(ctx context.Context, containerName string) error {
	if !m.Available() {
		return errors.New("lxc not available")
	}
	quickCtx, cancelQ := context.WithTimeout(ctx, queryTimeout)
	defer cancelQ()
	if _, err := lxcRun(quickCtx, "exec", containerName, "--", "which", "claude"); err == nil {
		return nil
	}

	installCtx, cancelI := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelI()
	script := `set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl ca-certificates gnupg
curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null 2>&1
apt-get install -y -qq nodejs
npm install -g @anthropic-ai/claude-code --silent 2>&1 | tail -3
which claude && claude --version`
	out, err := lxcRun(installCtx, "exec", containerName, "--", "bash", "-c", script)
	if err != nil {
		return fmt.Errorf("install claude in %s: %w; output: %s",
			containerName, err, truncateOut(out, 1000))
	}
	return nil
}

func truncateOut(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func (m *Manager) Stop(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	if out, err := lxcRun(lctx, "stop", containerName); err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "is already stopped") {
			return nil
		}
		return fmt.Errorf("lxc stop: %w; output: %s", err, out)
	}
	return nil
}

func (m *Manager) Delete(ctx context.Context, containerName string) error {
	if !m.Available() {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	if out, err := lxcRun(lctx, "delete", "--force", containerName); err != nil {
		if strings.Contains(out, "not found") {
			return nil
		}
		return fmt.Errorf("lxc delete: %w; output: %s", err, out)
	}
	return nil
}

func (m *Manager) State(ctx context.Context, containerName string) (serviceproject.ContainerState, error) {
	if !m.Available() {
		return serviceproject.ContainerStateUnknown, nil
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := lxcRun(lctx, "info", containerName)
	if err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "doesn't exist") {
			return serviceproject.ContainerStateMissing, nil
		}
		return serviceproject.ContainerStateUnknown, fmt.Errorf("lxc info: %w; output: %s", err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Status:") {
			s := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			switch strings.ToUpper(s) {
			case "RUNNING":
				return serviceproject.ContainerStateRunning, nil
			case "STOPPED":
				return serviceproject.ContainerStateStopped, nil
			case "FROZEN":
				return serviceproject.ContainerStateFrozen, nil
			default:
				return serviceproject.ContainerStateUnknown, nil
			}
		}
	}
	return serviceproject.ContainerStateUnknown, nil
}

func lxcRun(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "lxc", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func chownRecursive(root string, uid, gid int) error {
	return walkAndChown(root, uid, gid)
}

func walkAndChown(path string, uid, gid int) error {
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := walkAndChown(path+"/"+e.Name(), uid, gid); err != nil {
			return err
		}
	}
	return nil
}
