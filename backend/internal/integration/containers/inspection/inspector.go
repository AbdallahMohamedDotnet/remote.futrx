// Package inspection assembles best-effort diagnostic snapshots of project
// containers.
package inspection

// Inspect gathers a rich debugging snapshot of one project's container.
// Sources, in order:
//   1. `lxc query /1.0/instances/<n>`        — instance config (image, devices, limits)
//   2. `lxc query /1.0/instances/<n>/state`  — live runtime stats (memory, network, cpu)
//   3. `lxc exec <n> -- sh -c "<probe>"`     — OS info + df from inside the container
//   4. host-side stat()                       — per-bundle auth file mtimes
// Each section is best-effort: a stopped or missing container leaves
// dependent fields zero-valued.

import (
	"context"
	"time"

	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/agent/provisioning"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/assets"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/command"
	"github.com/Kings-Of-The-Web/remote.futrx.dev/internal/integration/containers/profiles"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

const inspectQuickTimeout = 5 * time.Second

// containerInspector owns the best-effort snapshot assembled from LXD, the
// guest operating system, configured agent profiles, and host credential files.
type Inspector struct {
	states      containerStateReader
	lxd         containerLXDInspector
	guest       containerGuestInspector
	agents      containerAgentInspector
	credentials containerCredentialInspector
}

type containerStateReader interface {
	State(ctx context.Context, containerName string) (serviceproject.ContainerState, error)
}

// NewInspector returns a diagnostic inspector backed by the lifecycle state
// reader and shared agent profiles.
func NewInspector(runner command.Runner, registry *profiles.Registry, states containerStateReader) *Inspector {
	commands := &quickCommandRunner{runner: runner, timeout: inspectQuickTimeout}
	return &Inspector{
		states: states,
		lxd:    containerLXDInspector{commands: commands},
		guest:  containerGuestInspector{commands: commands},
		agents: containerAgentInspector{
			commands:        commands,
			profiles:        registry,
			instructionHash: assets.Hash(provisioning.InstructionsTemplate()),
		},
		credentials: containerCredentialInspector{commands: commands, profiles: registry},
	}
}

func (i *Inspector) Inspect(ctx context.Context, containerName string) (serviceproject.ContainerInspect, error) {
	out := serviceproject.ContainerInspect{Name: containerName}

	state, err := i.states.State(ctx, containerName)
	if err != nil {
		return out, err
	}
	out.State = state
	if state == serviceproject.ContainerStateMissing {
		return out, nil
	}

	i.lxd.inspectConfiguration(ctx, containerName, &out)

	if state == serviceproject.ContainerStateRunning {
		i.lxd.inspectRuntime(ctx, containerName, &out)
		osInfo, disks := i.guest.inspect(ctx, containerName)
		out.OS = osInfo
		out.Disks = disks
		out.SetAgentStatuses(i.agents.inspect(ctx, containerName))
	}

	out.AuthBundles = i.credentials.inspect(ctx, containerName, state)
	return out, nil
}
