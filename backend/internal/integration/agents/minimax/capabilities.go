package minimax

import (
	"context"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func (p *Provider) Capabilities(context.Context, agent.CapabilityRequest) (agent.Capabilities, error) {
	reasoning := []agent.CapabilityOption{
		agent.AutoOption(),
		{Value: miniMaxReasoningDisabled, Label: "Think-Off", Description: "Disable Adaptive Thinking"},
		{Value: miniMaxReasoningAdaptive, Label: "Adaptive", Description: "Enable Adaptive Thinking"},
	}
	models := agent.WithAutoModel([]agent.ModelCapability{{
		ID:                     miniMaxModel,
		Label:                  miniMaxModel,
		Description:            "MiniMax M3 with a 1,000,000-token context window",
		ProviderDefault:        true,
		ReasoningEfforts:       reasoning,
		DefaultReasoningEffort: miniMaxReasoningAdaptive,
	}}, "MiniMax default")

	return agent.Capabilities{
		Provider:    agent.ProviderMiniMax,
		Label:       miniMaxLabel,
		Source:      agent.CapabilitySourceFallback,
		Models:      models,
		Modes:       agent.ProviderModes(true),
		DefaultMode: agent.RunModeDefault,
	}, nil
}

func miniMaxReasoningEffort(effort agent.ReasoningEffort) agent.ReasoningEffort {
	if strings.EqualFold(strings.TrimSpace(string(effort)), miniMaxReasoningDisabled) {
		return miniMaxReasoningDisabled
	}
	return miniMaxReasoningAdaptive
}
