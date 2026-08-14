package antigravity

import (
	"context"
	"fmt"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const capabilityTimeout = 10 * time.Second

func (p *Provider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	probeCtx, cancel := context.WithTimeout(ctx, capabilityTimeout)
	defer cancel()
	environment := []string{"HOME=" + containerAgentHome}

	modelsCmd := agent.NewCapabilityCommand(probeCtx, req, environment, "agy", "models")
	modelsOutput, modelsErr := modelsCmd.CombinedOutput()
	helpCmd := agent.NewCapabilityCommand(probeCtx, req, environment, "agy", "--help")
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
		Modes:       agent.CodeAndPlanModes(true),
		DefaultMode: "code",
	}
}
