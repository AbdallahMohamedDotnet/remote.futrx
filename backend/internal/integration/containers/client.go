package containers

// Client adapts LXD container operations for project workspaces.
// It builds on the thin internal/integration/lxc Client to do the actual
// `lxc <...>` invocations and applies the provider-neutral provisioning
// profiles supplied by the service composition root.

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
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

// Client implements the container ports consumed by project and prompt
// services. Wire it once at the composition root and share the pointer.
type Client struct {
	lxc         CommandRunner
	image       string
	profiles    profileRegistry
	templates   templatePublisher
	inspector   containerInspector
	credentials credentialSynchronizer
}

// New returns a Client that delegates CLI calls to the supplied runner.
func New(client CommandRunner) *Client {
	containerClient := &Client{
		lxc:       client,
		image:     defaultImage,
		templates: templatePublisher{lxc: client},
	}
	containerClient.inspector = containerInspector{
		lxc:      client,
		profiles: &containerClient.profiles,
		states:   containerClient,
	}
	containerClient.credentials = credentialSynchronizer{
		lxc:      client,
		profiles: &containerClient.profiles,
	}
	return containerClient
}

func (c *Client) ConfigureAgentProfiles(profiles []provisioning.Profile) {
	c.profiles.replace(profiles)
}

func (c *Client) AgentProfiles() []provisioning.Profile {
	return c.profiles.snapshot()
}

// Available reports whether the underlying lxc binary is reachable.
func (c *Client) Available() bool { return c.lxc.Available() }

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
