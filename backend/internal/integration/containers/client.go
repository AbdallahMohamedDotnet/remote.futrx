package containers

// Client adapts LXD container operations for project workspaces.
// It builds on the thin internal/integration/lxc Client to do the actual
// `lxc <...>` invocations and applies the provider-neutral provisioning
// profiles supplied by the service composition root.

import (
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/output"
	profileconfig "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/profiles"
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
	lxc           CommandRunner
	profiles      *profileRegistry
	templates     *templatePublisher
	inspector     containerInspector
	credentials   credentialSynchronizer
	clis          cliProvisioner
	browser       agentBrowser
	browserMCP    agentBrowserMCPProvisioner
	browserConfig agentBrowserConfigurator
	codeServer    codeServerProvisioner
	lifecycle     containerLifecycle
	workspace     workspaceProvisioner
	images        baseImageBuilder
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
	containerClient.credentials = credentialSynchronizer{
		profiles: containerClient.profiles,
		files:    credentialFileSynchronizer{lxc: client},
	}
	containerClient.credentials.directories = credentialDirectorySynchronizer{
		lxc:   client,
		files: &containerClient.credentials.files,
	}
	containerClient.clis = cliProvisioner{
		lxc:      client,
		profiles: containerClient.profiles,
	}
	containerClient.browser = agentBrowser{
		provisioner: agentBrowserProvisioner{lxc: client, templates: containerClient.templates},
		runtime:     agentBrowserRuntime{lxc: client},
	}
	containerClient.browserMCP = agentBrowserMCPProvisioner{
		lxc:       client,
		profiles:  containerClient.profiles,
		templates: containerClient.templates,
	}
	containerClient.browserConfig = agentBrowserConfigurator{lxc: client}
	containerClient.codeServer = codeServerProvisioner{lxc: client}
	containerClient.workspace = workspaceProvisioner{
		lxc:       client,
		profiles:  containerClient.profiles,
		templates: containerClient.templates,
	}
	containerClient.images = baseImageBuilder{
		lxc:      client,
		profiles: containerClient.profiles,
	}
	containerClient.lifecycle.provisioner = &containerLaunchProvisioner{
		credentials: &containerClient.credentials,
		workspace:   &containerClient.workspace,
		browser:     &containerClient.browserConfig,
		codeServer:  &containerClient.codeServer,
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
