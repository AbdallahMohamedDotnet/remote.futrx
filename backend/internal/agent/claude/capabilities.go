package claude

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const capabilityTimeout = 15 * time.Second
const fastServiceTier = "fast"

func (p *Provider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	probeCtx, cancel := context.WithTimeout(ctx, capabilityTimeout)
	defer cancel()

	catalog, fullyResolved, catalogErr := queryModelCatalog(probeCtx, req)
	helpCmd := agent.NewCapabilityCommand(
		probeCtx,
		req,
		[]string{"HOME=/root", "IS_SANDBOX=1"},
		"claude",
		"--help",
	)
	helpOutput, helpErr := helpCmd.CombinedOutput()
	if catalogErr != nil {
		caps := fallbackCapabilities()
		caps.Warning = "Claude model catalog could not be read from the CLI"
		return caps, fmt.Errorf("claude capability discovery: %w", catalogErr)
	}

	reasoning := parseHelpEfforts(string(helpOutput))
	caps := buildCapabilities(catalog, reasoning)
	if helpErr != nil || len(reasoning) == 0 {
		caps.Warning = "Claude effort levels could not be read from the CLI"
	}
	if !fullyResolved {
		caps.Warning = "Some Claude model versions could not be resolved by the CLI"
	}
	return caps, nil
}

func fallbackCapabilities() agent.Capabilities {
	return buildCapabilities(fallbackModelCatalog(), fallbackReasoningOptions())
}

func buildCapabilities(catalog claudeModelCatalog, reasoning []agent.CapabilityOption) agent.Capabilities {
	if len(reasoning) == 0 {
		reasoning = fallbackReasoningOptions()
	}
	models := make([]agent.ModelCapability, 0, len(catalog.Selections)+1)
	models = append(models, agent.ModelCapability{
		ID:               "",
		Label:            autoModelLabel(catalog.DefaultLabel),
		Description:      "Use the model selected by Claude Code for this account",
		ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		ServiceTiers:     fastModeOptions(),
	})
	for _, selection := range catalog.Selections {
		model := agent.ModelCapability{
			ID:               selection.ID,
			Label:            selection.Label,
			Description:      selection.Description,
			ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		}
		if supportsFastMode(selection.ID) {
			model.ServiceTiers = fastModeOptions()
		}
		models = append(models, model)
	}
	return agent.Capabilities{
		Provider:    agent.ProviderClaude,
		Label:       "Claude",
		Source:      catalog.Source,
		Models:      models,
		Modes:       agent.ProviderModes(true),
		DefaultMode: agent.RunModeDefault,
	}
}

func fallbackReasoningOptions() []agent.CapabilityOption {
	reasoning := []agent.CapabilityOption{agent.AutoOption()}
	for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
		reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: optionLabel(effort)})
	}
	return reasoning
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
	model = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(model)), "[1m]")
	return model == "opus"
}

func autoModelLabel(resolved string) string {
	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return "Auto"
	}
	return "Auto · " + resolved
}
