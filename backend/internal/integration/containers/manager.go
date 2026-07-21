package containers

// Manager owns the lifecycle of LXD containers used for project workspaces.
// It builds on the thin internal/integration/lxc Client to do the actual
// `lxc <...>` invocations and layers policy on top: workspace bind mounts,
// boot-autostart, an auth-bundle pipeline for shipping host-side OAuth
// credentials into containers, and per-provider provisioning (Claude CLI
// install, CLAUDE.md template).

import (
	"context"
	"io"
	"os"
	"sync"
	"time"
)

const (
	// hostMappedUID is the host uid that LXD's default idmap presents as
	// root inside an unprivileged container. Files in the bind-mounted
	// workspace must be owned by this uid or the container's root cannot
	// write them.
	hostMappedUID = 1000000

	defaultImage = BaseImageAlias
	containerWS  = "/workspace"

	launchTimeout = 90 * time.Second
	startTimeout  = 30 * time.Second
	stopTimeout   = 30 * time.Second
	deleteTimeout = 30 * time.Second
	queryTimeout  = 10 * time.Second
)

// CommandRunner is the transport seam used to invoke the container runtime.
// The LXC CLI adapter implements it at the application composition root.
type CommandRunner interface {
	Available() bool
	Run(ctx context.Context, args ...string) (string, error)
	RunStdin(ctx context.Context, stdin io.Reader, args ...string) (string, error)
}

// Manager is the value passed to services that need to launch or drive
// containers. Wire it up once in main and share the pointer.
type Manager struct {
	lxc   CommandRunner
	image string

	mu      sync.RWMutex
	bundles []AuthBundle
}

// New returns a Manager that delegates CLI calls to the supplied runner.
func New(client CommandRunner) *Manager {
	return &Manager{lxc: client, image: defaultImage}
}

// Available reports whether the underlying lxc binary is reachable.
func (m *Manager) Available() bool { return m.lxc.Available() }

func truncateOut(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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
