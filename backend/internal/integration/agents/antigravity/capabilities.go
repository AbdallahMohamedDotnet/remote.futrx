package antigravity

import (
	"context"
	"fmt"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	agentruntime "github.com/futrx-com/remote.futrx.com/internal/integration/agents/runtime"
)

func (p *Provider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	environment := []string{"HOME=" + containerAgentHome}

	// Keep model discovery machine-readable. In agy 1.1.x output-format is a
	// global flag and the models payload is nested under command.data.models.
	// Output intentionally excludes stderr: agy writes a human progress line to
	// stderr even while stdout contains valid JSON.
	modelsCmd := agentruntime.NewCapabilityCommand(ctx, req, environment, "agy", "--output-format", "json", "models")
	modelsOutput, modelsErr := modelsCmd.Output()
	helpCmd := agentruntime.NewCapabilityCommand(ctx, req, environment, "agy", "--help")
	helpOutput, helpErr := helpCmd.CombinedOutput()

	if modelsErr != nil && helpErr != nil {
		caps := fallbackCapabilities()
		caps.Warning = "Antigravity capabilities could not be read from the CLI"
		return caps, fmt.Errorf("antigravity capability discovery: models: %v; help: %w", modelsErr, helpErr)
	}
	caps := parseCLIOutputCatalog(string(modelsOutput), string(helpOutput))
	if modelsErr != nil {
		caps.Source = agent.CapabilitySourceFallback
		caps.Warning = "Sign in to Antigravity in this project to load its model catalog"
		caps.UnavailableReason = "Antigravity is not signed in on the host."
		if req.ContainerName != "" {
			caps.UnavailableReason = "Sign in to Antigravity in this project's terminal, then refresh models."
		}
		return caps, modelsErr
	}
	return caps, nil
}

func fallbackCapabilities() agent.Capabilities {
	reasoning := []agent.CapabilityOption{agent.AutoOption()}
	for _, effort := range []string{"low", "medium", "high"} {
		reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: capabilityLabel(effort)})
	}
	models := agent.WithAutoModel(nil, "Antigravity default")
	models[0].ReasoningEfforts = reasoning
	return agent.Capabilities{
		Provider:    agent.ProviderAntigravity,
		Label:       "Antigravity",
		Source:      agent.CapabilitySourceFallback,
		Models:      models,
		Modes:       agent.ProviderModes(false),
		DefaultMode: agent.RunModeDefault,
	}
}
