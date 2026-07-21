package containers

// Client adapts LXD container operations for project workspaces.
// It builds on the thin internal/integration/lxc Client to do the actual
// `lxc <...>` invocations and applies the provider-neutral provisioning
// profiles supplied by the service composition root.

import (
	"context"
	"io"
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
	profiles    profileRegistry
	templates   templatePublisher
	inspector   containerInspector
	credentials credentialSynchronizer
	clis        cliProvisioner
	browser     agentBrowser
	browserMCP  agentBrowserMCPProvisioner
	codeServer  codeServerProvisioner
	lifecycle   containerLifecycle
	workspace   workspaceProvisioner
	images      baseImageBuilder
}

// New returns a Client that delegates CLI calls to the supplied runner.
func New(client CommandRunner) *Client {
	containerClient := &Client{
		lxc:       client,
		templates: templatePublisher{lxc: client},
	}
	containerClient.lifecycle = containerLifecycle{
		lxc:       client,
		image:     defaultImage,
		workspace: hostWorkspacePreparer{uid: hostMappedUID, gid: hostMappedUID},
	}
	inspectionCommands := &quickCommandRunner{lxc: client, timeout: inspectQuickTimeout}
	containerClient.inspector = containerInspector{
		states:      containerClient,
		lxd:         containerLXDInspector{commands: inspectionCommands},
		guest:       containerGuestInspector{commands: inspectionCommands},
		agents:      containerAgentInspector{commands: inspectionCommands, profiles: &containerClient.profiles},
		credentials: containerCredentialInspector{commands: inspectionCommands, profiles: &containerClient.profiles},
	}
	containerClient.credentials = credentialSynchronizer{
		profiles: &containerClient.profiles,
		files:    credentialFileSynchronizer{lxc: client},
	}
	containerClient.credentials.directories = credentialDirectorySynchronizer{
		lxc:   client,
		files: &containerClient.credentials.files,
	}
	containerClient.clis = cliProvisioner{
		lxc:      client,
		profiles: &containerClient.profiles,
	}
	containerClient.browser = agentBrowser{
		lxc:       client,
		templates: &containerClient.templates,
	}
	containerClient.browserMCP = agentBrowserMCPProvisioner{
		lxc:       client,
		profiles:  &containerClient.profiles,
		templates: &containerClient.templates,
	}
	containerClient.codeServer = codeServerProvisioner{lxc: client}
	containerClient.workspace = workspaceProvisioner{
		lxc:       client,
		profiles:  &containerClient.profiles,
		templates: &containerClient.templates,
	}
	containerClient.images = baseImageBuilder{
		lxc:      client,
		profiles: &containerClient.profiles,
	}
	containerClient.lifecycle.provisioner = &containerLaunchProvisioner{
		credentials: &containerClient.credentials,
		workspace:   &containerClient.workspace,
		browser:     &containerClient.browser,
		codeServer:  &containerClient.codeServer,
	}
	return containerClient
}

func (c *Client) ConfigureAgentProfiles(profiles []provisioning.Profile) {
	c.profiles.replace(profiles)
}

// Available reports whether the underlying lxc binary is reachable.
func (c *Client) Available() bool { return c.lxc.Available() }

func truncateOut(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
