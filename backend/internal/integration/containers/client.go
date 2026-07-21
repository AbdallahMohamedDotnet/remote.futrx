package containers

// Client adapts LXD container operations for project workspaces.
// It builds on the thin internal/integration/lxc Client to do the actual
// `lxc <...>` invocations and applies the provider-neutral provisioning
// profiles supplied by the service composition root.

import (
	"context"
	"io"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	containerbaseimage "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/baseimage"
	containerbrowser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/browser"
	containercli "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/cli"
	containercodeserver "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/codeserver"
	containercredentials "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/credentials"
	containerenvironment "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/environment"
	containerinspection "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/inspection"
	containerlaunch "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/launch"
	containerlifecycle "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/lifecycle"
	containerlisteners "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/listeners"
	containernetwork "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/network"
	profileconfig "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/profiles"
	containerworkspace "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/workspace"
)

const defaultImage = containerbaseimage.Alias

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
	profiles    *profileconfig.Registry
	templates   *assets.Publisher
	inspector   *containerinspection.Inspector
	credentials *containercredentials.Synchronizer
	environment *containerenvironment.Service
	clis        *containercli.Provisioner
	browser     *containerbrowser.Service
	codeServer  *containercodeserver.Provisioner
	lifecycle   *containerlifecycle.Service
	listeners   *containerlisteners.Scanner
	network     *containernetwork.Repairer
	workspace   *containerworkspace.Provisioner
	images      *containerbaseimage.Builder
}

// New returns a Client that delegates CLI calls to the supplied runner.
func New(client CommandRunner) *Client {
	containerClient := &Client{
		lxc:       client,
		profiles:  profileconfig.NewRegistry(),
		templates: assets.NewPublisher(client),
	}
	containerClient.credentials = containercredentials.NewSynchronizer(client, containerClient.profiles)
	containerClient.environment = containerenvironment.NewService(client)
	containerClient.listeners = containerlisteners.NewScanner(client)
	containerClient.network = containernetwork.NewRepairer(client)
	containerClient.clis = containercli.NewProvisioner(client, containerClient.profiles, containerbaseimage.InstallScript)
	containerClient.browser = containerbrowser.NewService(client, containerClient.profiles, containerClient.templates)
	containerClient.codeServer = containercodeserver.NewProvisioner(client)
	containerClient.workspace = containerworkspace.NewProvisioner(client, containerClient.profiles, containerClient.templates)
	containerClient.images = containerbaseimage.NewBuilder(
		client,
		containerClient.profiles,
		containerbrowser.InstallScript(),
		containercodeserver.InstallScript(),
	)
	launchProvisioner := containerlaunch.NewProvisioner(
		containerClient.credentials,
		containerClient.workspace,
		containerClient.browser,
		containerClient.codeServer,
	)
	containerClient.lifecycle = containerlifecycle.NewService(client, defaultImage, launchProvisioner)
	containerClient.inspector = containerinspection.NewInspector(client, containerClient.profiles, containerClient.lifecycle)
	return containerClient
}

func (c *Client) ConfigureAgentProfiles(profiles []provisioning.Profile) {
	c.profiles.Replace(profiles)
}

// Available reports whether the underlying lxc binary is reachable.
func (c *Client) Available() bool { return c.lxc.Available() }
