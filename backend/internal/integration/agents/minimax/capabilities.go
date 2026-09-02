package minimax

import (
	"context"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	configconstants "github.com/futrx-com/remote.futrx.com/internal/config/constants"
)

func (p *Provider) Capabilities(context.Context, agent.CapabilityRequest) (agent.Capabilities, error) {
	reasoning := []agent.CapabilityOption{
		agent.AutoOption(),
		{
			Value:       configconstants.MiniMaxReasoningDisabled,
			Label:       configconstants.MiniMaxReasoningDisabledLabel,
			Description: configconstants.MiniMaxReasoningDisabledDescription,
		},
		{
			Value:       configconstants.MiniMaxReasoningAdaptive,
			Label:       configconstants.MiniMaxReasoningAdaptiveLabel,
			Description: configconstants.MiniMaxReasoningAdaptiveDescription,
		},
	}
	models := agent.WithAutoModel([]agent.ModelCapability{{
		ID:                     configconstants.MiniMaxModel,
		Label:                  configconstants.MiniMaxModel,
		Description:            configconstants.MiniMaxModelDescription,
		ProviderDefault:        true,
		ReasoningEfforts:       reasoning,
		DefaultReasoningEffort: configconstants.MiniMaxReasoningAdaptive,
	}}, configconstants.MiniMaxAutoModelLabel)

	return agent.Capabilities{
		Provider:    agent.ProviderMiniMax,
		Label:       configconstants.MiniMaxLabel,
		Source:      agent.CapabilitySourceFallback,
		Models:      models,
		Modes:       agent.ProviderModes(true),
		DefaultMode: agent.RunModeDefault,
	}, nil
}

func miniMaxReasoningEffort(effort agent.ReasoningEffort) agent.ReasoningEffort {
	if strings.EqualFold(strings.TrimSpace(string(effort)), configconstants.MiniMaxReasoningDisabled) {
		return configconstants.MiniMaxReasoningDisabled
	}
	return configconstants.MiniMaxReasoningAdaptive
}
