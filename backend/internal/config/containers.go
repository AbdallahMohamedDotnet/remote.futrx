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
	servicebrowser "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/browser"
	servicecredentials "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/credentials"
	serviceinspection "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/inspection"
	containerlaunch "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/launch"
	servicelifecycle "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/lifecycle"
	serviceprofiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/profiles"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const hostMappedUID = 1000000

// ContainerStack is the composition root for container application services
// and their LXD/host-filesystem adapters.
type ContainerStack struct {
	Lifecycle   *servicelifecycle.Service
	Inspection  *serviceinspection.Service
	Credentials *servicecredentials.Service
	Environment *containerenvironment.Service
	CLI         *containercli.Provisioner
	Browser     *servicebrowser.Service
	Listeners   *containerlisteners.Scanner
	Network     *containernetwork.Repairer
	Workspace   *containerworkspace.Provisioner
	Images      *containerbaseimage.Builder
}

// ProjectDependencies exposes only the capabilities consumed by project
// policy. Each capability can be replaced independently in tests or by a
// different runtime adapter.
func (s ContainerStack) ProjectDependencies() serviceproject.ContainerDependencies {
	return serviceproject.ContainerDependencies{
		Lifecycle:   s.Lifecycle,
		Environment: s.Environment,
		Inspector:   s.Inspection,
		Network:     s.Network,
		Listeners:   s.Listeners,
		Browser:     s.Browser,
	}
}

// AgentDependencies exposes only the capabilities used while preparing a
// container for an agent provider.
func (s ContainerStack) AgentDependencies() provisioning.ContainerDependencies {
	return provisioning.ContainerDependencies{
		CLI:         s.CLI,
		Credentials: s.Credentials,
		Workspace:   s.Workspace,
		Browser:     s.Browser,
		Lifecycle:   s.Lifecycle,
	}
}

func NewContainerStack(runner command.Runner, configuredProfiles []provisioning.Profile) ContainerStack {
	profiles := serviceprofiles.NewCatalog(configuredProfiles)
	publisher := assets.NewPublisher(runner)
	credentialTransfer := containercredentials.NewAdapter(runner)
	credentials := servicecredentials.NewService(profiles, credentialTransfer)
	environment := containerenvironment.NewService(runner)
	listeners := containerlisteners.NewScanner(runner)
	network := containernetwork.NewRepairer(runner)
	cli := containercli.NewProvisioner(runner, profiles, containerbaseimage.InstallScript)
	browserAdapter := containerbrowser.NewAdapter(runner, profiles, publisher)
	browser := servicebrowser.NewService(servicebrowser.Dependencies{
		Provisioner: browserAdapter,
		Runtime:     browserAdapter,
		Tooling:     browserAdapter,
	}, containerbrowser.VNCPort)
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
	inspectionAdapter := containerinspection.NewAdapter(runner, profiles)
	inspection := serviceinspection.NewService(serviceinspection.Dependencies{
		States:        lifecycle,
		Configuration: inspectionAdapter,
		Runtime:       inspectionAdapter,
		Guest:         inspectionAdapter,
		Agents:        inspectionAdapter,
		Credentials:   inspectionAdapter,
	})

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
