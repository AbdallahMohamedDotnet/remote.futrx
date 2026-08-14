package claude

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const capabilityTimeout = 8 * time.Second
const fastServiceTier = "fast"

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
		model := agent.ModelCapability{
			ID: id, Label: optionLabel(id), ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		}
		if supportsFastMode(id) {
			model.ServiceTiers = fastModeOptions()
		}
		models = append(models, model)
	}
	return agent.Capabilities{
		Provider:    agent.ProviderClaude,
		Label:       "Claude",
		Source:      agent.CapabilitySourceFallback,
		Models:      withAutoFastMode(models),
		Modes:       agent.ProviderModes(true),
		DefaultMode: agent.RunModeDefault,
	}
}

func withAutoFastMode(models []agent.ModelCapability) []agent.ModelCapability {
	models = agent.WithAutoModel(models, "Claude default")
	models[0].ServiceTiers = fastModeOptions()
	return models
}

func fastModeOptions() []agent.CapabilityOption {
	return []agent.CapabilityOption{
		agent.AutoOption(),
		{
			Value:       fastServiceTier,
			Label:       "Fast",
			Description: "Use Claude Fast mode for lower latency at a higher token cost",
		},
	}
}

func supportsFastMode(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "opus")
}
