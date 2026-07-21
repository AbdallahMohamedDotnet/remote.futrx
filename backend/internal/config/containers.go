package config

import (
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	containerbaseimage "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/baseimage"
	containerbrowser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/browser"
	containercli "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/cli"
	containercodeserver "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/codeserver"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	containercredentials "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/credentials"
	containerenvironment "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/environment"
	containerinspection "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/inspection"
	containerlifecycle "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/lifecycle"
	containerlisteners "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/listeners"
	containernetwork "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/network"
	containerresources "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/resources"
	containerworkspace "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/workspace"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/hostfs"
	containerlaunch "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/launch"
	servicelifecycle "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/lifecycle"
	serviceprofiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/profiles"
)

const hostMappedUID = 1000000

// ContainerStack is the composition root for container application services
// and their LXD/host-filesystem adapters.
type ContainerStack struct {
	Lifecycle   *servicelifecycle.Service
	Inspection  *containerinspection.Inspector
	Credentials *containercredentials.Synchronizer
	Environment *containerenvironment.Service
	CLI         *containercli.Provisioner
	Browser     *containerbrowser.Service
	Listeners   *containerlisteners.Scanner
	Network     *containernetwork.Repairer
	Workspace   *containerworkspace.Provisioner
	Images      *containerbaseimage.Builder
}

func NewContainerStack(runner command.Runner, configuredProfiles []provisioning.Profile) ContainerStack {
	profiles := serviceprofiles.NewCatalog(configuredProfiles)
	publisher := assets.NewPublisher(runner)
	credentials := containercredentials.NewSynchronizer(runner, profiles)
	environment := containerenvironment.NewService(runner)
	listeners := containerlisteners.NewScanner(runner)
	network := containernetwork.NewRepairer(runner)
	cli := containercli.NewProvisioner(runner, profiles, containerbaseimage.InstallScript)
	browser := containerbrowser.NewService(runner, profiles, publisher)
	codeServer := containercodeserver.NewProvisioner(runner)
	workspace := containerworkspace.NewProvisioner(runner, profiles, publisher)
	images := containerbaseimage.NewBuilder(
		runner,
		profiles,
		containerbrowser.InstallScript(),
		containercodeserver.InstallScript(),
	)
	launchProvisioner := containerlaunch.NewProvisioner(
		credentials,
		workspace,
		browser,
		codeServer,
	)
	resources := containerresources.NewManager(runner)
	lifecycle := servicelifecycle.NewService(
		containerlifecycle.NewClient(runner),
		containerbaseimage.Alias,
		hostfs.NewWorkspacePreparer(hostMappedUID, hostMappedUID),
		resources,
		launchProvisioner,
	)
	inspection := containerinspection.NewInspector(runner, profiles, lifecycle)

	return ContainerStack{
		Lifecycle:   lifecycle,
		Inspection:  inspection,
		Credentials: credentials,
		Environment: environment,
		CLI:         cli,
		Browser:     browser,
		Listeners:   listeners,
		Network:     network,
		Workspace:   workspace,
		Images:      images,
	}
}
