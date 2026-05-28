package projects

// Wrapper around the `lxc` CLI to launch / inspect / delete project containers.
//
// Design notes:
//   - We shell out to `lxc` instead of using LXD's REST API directly. Reasons:
//     stable CLI surface, easy to debug locally (the same commands work in a
//     shell), no extra Go dependency. Performance: each lxc call is a few ms
//     of fork/exec overhead which is fine for our scale.
//   - Containers are unprivileged (LXD default). Container's uid 0 maps to
//     host's uid 1000000. To make bind-mounted workspaces writable from
//     inside the container, we chown the workspace dir to uid 1000000:1000000
//     on the host. Host root can still read/write (root bypasses perms).
//   - We bind-mount /root/.claude read-only so all containers share the host's
//     Anthropic auth without a per-container login.
//   - Image is vanilla ubuntu:24.04. claude CLI + dev tools are installed by
//     claude itself on the first prompt of a project (task #9).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// hostMappedUID is the host UID that container UID 0 maps to under LXD's
	// default unprivileged config. We chown bind-mounted dirs to this so the
	// container's root can read/write them.
	hostMappedUID = 1000000

	defaultImage    = "ubuntu:24.04"
	hostClaudePath  = "/root/.claude"
	containerWS     = "/workspace"
	containerClaude = "/root/.claude"

	// Operation timeouts. Launching an unprivileged Ubuntu container takes
	// 5–10 seconds the first time the image is cached.
	launchTimeout  = 90 * time.Second
	startTimeout   = 30 * time.Second
	stopTimeout    = 30 * time.Second
	deleteTimeout  = 30 * time.Second
	queryTimeout   = 10 * time.Second
	execTimeoutDef = 60 * time.Second
)

// ContainerState matches what `lxc info` reports.
type ContainerState string

const (
	StateRunning ContainerState = "RUNNING"
	StateStopped ContainerState = "STOPPED"
	StateFrozen  ContainerState = "FROZEN"
	StateMissing ContainerState = "MISSING" // we map "container not found" to this for callers
	StateUnknown ContainerState = "UNKNOWN"
)

// Manager wraps the lxc CLI. Stateless — every call shells out.
type Manager struct {
	image string
}

func NewManager() *Manager {
	return &Manager{image: defaultImage}
}

// Available reports whether the lxc binary is on PATH. Lets the server boot
// even if LXD isn't installed yet, surfacing a useful error per-request.
func (m *Manager) Available() bool {
	_, err := exec.LookPath("lxc")
	return err == nil
}

// Launch creates and starts a container for the given project. The workspace
// dir must already exist on the host. Idempotent-ish: if a container with the
// same name already exists, return that state without trying to re-launch.
func (m *Manager) Launch(ctx context.Context, p ProjectMeta) error {
	if !m.Available() {
		return errors.New("lxc CLI not found on PATH — install LXD on the host first")
	}

	// Ensure workspace is writable by container's mapped root.
	if err := os.MkdirAll(p.Cwd, 0o755); err != nil {
		return fmt.Errorf("create workspace dir: %w", err)
	}
	if err := chownRecursive(p.Cwd, hostMappedUID, hostMappedUID); err != nil {
		return fmt.Errorf("chown workspace: %w", err)
	}

	// Bail early if container already exists; caller can decide what to do.
	state, err := m.State(ctx, p.ContainerName)
	if err != nil {
		return err
	}
	if state != StateMissing {
		// Make sure it's running.
		if state == StateStopped {
			return m.Start(ctx, p.ContainerName)
		}
		return nil
	}

	// `lxc launch IMAGE NAME` creates + starts. Unprivileged by default.
	lctx, cancel := context.WithTimeout(ctx, launchTimeout)
	defer cancel()
	if out, err := lxcRun(lctx, "launch", m.image, p.ContainerName); err != nil {
		return fmt.Errorf("lxc launch: %w; output: %s", err, out)
	}

	// Attach the workspace bind-mount.
	if err := m.attachDisk(ctx, p.ContainerName, "workspace", p.Cwd, containerWS, false); err != nil {
		return fmt.Errorf("attach workspace: %w", err)
	}

	// Attach the read-only claude credentials, if the host has them.
	if info, err := os.Stat(hostClaudePath); err == nil && info.IsDir() {
		if err := m.attachDisk(ctx, p.ContainerName, "claude-auth", hostClaudePath, containerClaude, true); err != nil {
			// Not fatal — the user can still `claude auth login` inside.
			// Log via the returned error chain, but don't fail the launch.
			_ = err
		}
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

func (m *Manager) Start(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	if out, err := lxcRun(lctx, "start", containerName); err != nil {
		return fmt.Errorf("lxc start: %w; output: %s", err, out)
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context, containerName string) error {
	lctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	// --force-stop avoids waiting for graceful shutdown if it hangs.
	if out, err := lxcRun(lctx, "stop", containerName); err != nil {
		// Tolerate "already stopped" / not-found.
		if strings.Contains(out, "not found") || strings.Contains(out, "is already stopped") {
			return nil
		}
		return fmt.Errorf("lxc stop: %w; output: %s", err, out)
	}
	return nil
}

// Delete force-stops + deletes. Safe to call when container doesn't exist.
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

// State queries the live state of a container, mapping "not found" to Missing.
func (m *Manager) State(ctx context.Context, containerName string) (ContainerState, error) {
	if !m.Available() {
		return StateUnknown, nil
	}
	lctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	out, err := lxcRun(lctx, "info", containerName)
	if err != nil {
		if strings.Contains(out, "not found") || strings.Contains(out, "doesn't exist") {
			return StateMissing, nil
		}
		return StateUnknown, fmt.Errorf("lxc info: %w; output: %s", err, out)
	}
	// `lxc info` output starts with lines like "Name: x\nStatus: RUNNING\n...".
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Status:") {
			s := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			switch strings.ToUpper(s) {
			case "RUNNING":
				return StateRunning, nil
			case "STOPPED":
				return StateStopped, nil
			case "FROZEN":
				return StateFrozen, nil
			default:
				return StateUnknown, nil
			}
		}
	}
	return StateUnknown, nil
}

// --- helpers ---------------------------------------------------------------

func lxcRun(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "lxc", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// chownRecursive walks the tree under root and chowns each entry. Used to
// make the workspace dir owned by the container's mapped UID.
func chownRecursive(root string, uid, gid int) error {
	return walkAndChown(root, uid, gid)
}

// Tiny self-contained walker to avoid pulling filepath.Walk for one use.
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
