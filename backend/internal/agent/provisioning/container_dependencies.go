package provisioning

import (
	"context"
	"fmt"
	"strings"
)

// CLIProvisioner is the agent-execution port for making an agent CLI
// available inside a project container.
type CLIProvisioner interface {
	Ensure(context.Context, string, CLISpec) error
}

// CredentialSynchronizer is the agent-execution port for moving credentials
// into and out of a project container. Shared preparation owns the push path;
// providers that support post-run pull-back retain the same narrow port.
type CredentialSynchronizer interface {
	Ensure(context.Context, string, CredentialSpec) error
	SyncFromContainer(context.Context, string, CredentialSpec) error
}

// WorkspaceProvisioner publishes shared agent assets and workspace links.
type WorkspaceProvisioner interface {
	EnsureAgentInstructions(context.Context, string) error
	EnsureSkillLinks(context.Context, string) error
}

// BrowserProvisioner publishes browser tooling and starts its shared core.
type BrowserProvisioner interface {
	EnsureSkill(context.Context, string) error
	EnsureScript(context.Context, string) error
	EnsureMCP(context.Context, string) error
	EnsureCore(context.Context, string) error
}

// ScheduleToolsProvisioner publishes the provider-neutral schedule CLI and
// its selected skill into a project workspace.
type ScheduleToolsProvisioner interface {
	Ensure(context.Context, string) error
}

// ContainerLifecycle owns lifecycle settings needed by agent runs.
type ContainerLifecycle interface {
	EnsureBootAutostart(context.Context, string) error
}

// ContainerDependencies groups the focused ports used by shared agent project
// preparation and optional post-run credential sync. A zero value disables
// container preparation for host-only runs and focused tests.
type ContainerDependencies struct {
	CLI           CLIProvisioner
	Credentials   CredentialSynchronizer
	Workspace     WorkspaceProvisioner
	Browser       BrowserProvisioner
	ScheduleTools ScheduleToolsProvisioner
	Lifecycle     ContainerLifecycle
}

// IsZero reports whether no container provisioning ports were supplied.
func (d ContainerDependencies) IsZero() bool {
	return d.CLI == nil &&
		d.Credentials == nil &&
		d.Workspace == nil &&
		d.Browser == nil &&
		d.ScheduleTools == nil &&
		d.Lifecycle == nil
}

// Validate accepts either the zero value used by host-only providers or a
// complete set of container ports. Partial wiring is rejected before an agent
// preparation workflow can dereference a missing collaborator.
func (d ContainerDependencies) Validate() error {
	if d.IsZero() {
		return nil
	}

	missing := make([]string, 0, 6)
	if d.CLI == nil {
		missing = append(missing, "CLI")
	}
	if d.Credentials == nil {
		missing = append(missing, "credentials")
	}
	if d.Workspace == nil {
		missing = append(missing, "workspace")
	}
	if d.Browser == nil {
		missing = append(missing, "browser")
	}
	if d.ScheduleTools == nil {
		missing = append(missing, "schedule tools")
	}
	if d.Lifecycle == nil {
		missing = append(missing, "lifecycle")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("incomplete container dependencies: missing %s", strings.Join(missing, ", "))
}
