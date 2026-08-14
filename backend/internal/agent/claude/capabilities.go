package claude

import (
	"context"
	"fmt"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const capabilityTimeout = 8 * time.Second

func (p *Provider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	probeCtx, cancel := context.WithTimeout(ctx, capabilityTimeout)
	defer cancel()
	cmd := agent.NewCapabilityCommand(
		probeCtx,
		req,
		[]string{"HOME=/root", "IS_SANDBOX=1"},
		"claude",
		"--help",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		caps := fallbackCapabilities()
		caps.Warning = "Claude capabilities could not be read from the CLI"
		return caps, fmt.Errorf("claude capability discovery: %w", err)
	}
	return parseCapabilityHelp(string(output)), nil
}

func fallbackCapabilities() agent.Capabilities {
	reasoning := []agent.CapabilityOption{agent.AutoOption()}
	for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
		reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: optionLabel(effort)})
	}
	models := make([]agent.ModelCapability, 0, 4)
	for _, id := range []string{"fable", "opus", "sonnet", "haiku"} {
		models = append(models, agent.ModelCapability{
			ID: id, Label: optionLabel(id), ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		})
	}
	return agent.Capabilities{
		Provider:    agent.ProviderClaude,
		Label:       "Claude",
		Source:      agent.CapabilitySourceFallback,
		Models:      agent.WithAutoModel(models, "Claude default"),
		Modes:       agent.ProviderModes(true),
		DefaultMode: agent.RunModeDefault,
	}
}
