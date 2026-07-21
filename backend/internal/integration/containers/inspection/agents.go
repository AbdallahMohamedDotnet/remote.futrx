package inspection

import (
	"context"
	"strings"

	serviceprofiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/profiles"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

// containerAgentInspector reports provider CLI and instruction readiness.
type containerAgentInspector struct {
	commands        *quickCommandRunner
	profiles        serviceprofiles.Source
	instructionHash string
}

func (i *containerAgentInspector) inspect(ctx context.Context, containerName string) []serviceproject.AgentContainerStatus {
	profiles := i.profiles.Snapshot()
	statuses := make([]serviceproject.AgentContainerStatus, 0, len(profiles))
	for _, profile := range profiles {
		status := serviceproject.AgentContainerStatus{ID: profile.ID}
		if _, err := i.commands.run(ctx, "exec", containerName, "--", "which", profile.CLI.Binary); err == nil {
			status.Installed = true
			if version, err := i.commands.run(ctx, "exec", containerName, "--", profile.CLI.Binary, "--version"); err == nil {
				status.Version = strings.TrimSpace(version)
			}
		}
		if profile.Instructions != nil {
			if _, err := i.commands.run(ctx, "exec", containerName, "--", "test", "-f", profile.Instructions.Path); err == nil {
				status.InstructionsInstalled = true
			}
			if hash, err := i.commands.run(ctx, "exec", containerName, "--", "cat", profile.Instructions.HashPath); err == nil {
				status.InstructionsInSync = strings.TrimSpace(hash) == i.instructionHash
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}
