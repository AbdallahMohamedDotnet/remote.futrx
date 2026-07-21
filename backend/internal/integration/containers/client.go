package containers

// Client adapts LXD container operations for project workspaces.
// It builds on the thin internal/integration/lxc Client to do the actual
// `lxc <...>` invocations and applies the provider-neutral provisioning
// profiles supplied by the service composition root.

import (
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	containerbrowser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/browser"
	containercodeserver "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/codeserver"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	containercredentials "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/credentials"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/output"
	profileconfig "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/profiles"
	containerworkspace "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/workspace"
)

const (
	// hostMappedUID is the host uid that LXD's default idmap presents as
	// root inside an unprivileged container. Files in the bind-mounted
	// workspace must be owned by this uid or the container's root cannot
	// write them.
	hostMappedUID = 1000000

	defaultImage           = BaseImageAlias
	containerWorkspacePath = "/workspace"

	launchTimeout = 90 * time.Second
	startTimeout  = 30 * time.Second
	stopTimeout   = 30 * time.Second
	deleteTimeout = 30 * time.Second
	queryTimeout  = 10 * time.Second
)

// CommandRunner is the transport seam used to invoke the container runtime.
// The LXC CLI adapter implements it at the application composition root.
type CommandRunner = command.Runner

type profileRegistry = profileconfig.Registry
type templatePublisher = assets.Publisher

// Client implements the container ports consumed by project and prompt
// services. Wire it once at the composition root and share the pointer.
type Client struct {
	lxc         CommandRunner
	profiles    *profileRegistry
	templates   *templatePublisher
	inspector   containerInspector
	credentials *containercredentials.Synchronizer
	clis        cliProvisioner
	browser     *containerbrowser.Service
	codeServer  *containercodeserver.Provisioner
	lifecycle   containerLifecycle
	workspace   *containerworkspace.Provisioner
	images      baseImageBuilder
}

// New returns a Client that delegates CLI calls to the supplied runner.
func New(client CommandRunner) *Client {
	containerClient := &Client{
		lxc:       client,
		profiles:  profileconfig.NewRegistry(),
		templates: assets.NewPublisher(client),
	}
	containerClient.lifecycle = containerLifecycle{
		lxc:       client,
		image:     defaultImage,
		workspace: hostWorkspacePreparer{uid: hostMappedUID, gid: hostMappedUID},
	}
	inspectionCommands := &quickCommandRunner{lxc: client, timeout: inspectQuickTimeout}
	containerClient.inspector = containerInspector{
		states:      &containerClient.lifecycle,
		lxd:         containerLXDInspector{commands: inspectionCommands},
		guest:       containerGuestInspector{commands: inspectionCommands},
		agents:      containerAgentInspector{commands: inspectionCommands, profiles: containerClient.profiles},
		credentials: containerCredentialInspector{commands: inspectionCommands, profiles: containerClient.profiles},
	}
	containerClient.credentials = containercredentials.NewSynchronizer(client, containerClient.profiles)
	containerClient.clis = cliProvisioner{
		lxc:      client,
		profiles: containerClient.profiles,
	}
	containerClient.browser = containerbrowser.NewService(client, containerClient.profiles, containerClient.templates)
	containerClient.codeServer = containercodeserver.NewProvisioner(client)
	containerClient.workspace = containerworkspace.NewProvisioner(client, containerClient.profiles, containerClient.templates)
	containerClient.images = baseImageBuilder{
		lxc:      client,
		profiles: containerClient.profiles,
	}
	containerClient.lifecycle.provisioner = &containerLaunchProvisioner{
		credentials: containerClient.credentials,
		workspace:   containerClient.workspace,
		browser:     containerClient.browser,
		codeServer:  containerClient.codeServer,
	}
	return containerClient
}

func (c *Client) ConfigureAgentProfiles(profiles []provisioning.Profile) {
	c.profiles.Replace(profiles)
}

// Available reports whether the underlying lxc binary is reachable.
func (c *Client) Available() bool { return c.lxc.Available() }

func truncateOut(s string, max int) string {
	return output.Truncate(s, max)
}
