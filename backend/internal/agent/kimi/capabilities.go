package kimi

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

	kimiHome := containerKimiHome
	if req.ContainerName == "" {
		kimiHome = hostKimiHome()
	}
	modelsCmd := agent.NewCapabilityCommand(
		probeCtx,
		req,
		[]string{"HOME=/root", "KIMI_CODE_HOME=" + kimiHome},
		"kimi",
		"provider",
		"list",
		"--json",
	)
	modelsOutput, modelsErr := modelsCmd.Output()
	helpCmd := agent.NewCapabilityCommand(
		probeCtx,
		req,
		[]string{"HOME=/root", "KIMI_CODE_HOME=" + kimiHome},
		"kimi",
		"--help",
	)
	helpOutput, helpErr := helpCmd.CombinedOutput()

	if modelsErr != nil && helpErr != nil {
		caps := fallbackCapabilities()
		caps.Warning = "Kimi capabilities could not be read from the CLI"
		return caps, fmt.Errorf("kimi capability discovery: models: %v; help: %w", modelsErr, helpErr)
	}
	caps, err := parseProviderCatalog(modelsOutput, string(helpOutput))
	if err != nil {
		fallback := fallbackCapabilities()
		fallback.Warning = "Kimi returned an unreadable provider catalog"
		return fallback, err
	}
	return caps, nil
}

func fallbackCapabilities() agent.Capabilities {
	return agent.Capabilities{
		Provider:    agent.ProviderKimi,
		Label:       "Kimi",
		Source:      agent.CapabilitySourceFallback,
		Models:      agent.WithAutoModel(nil, "Kimi default"),
		Modes:       agent.ProviderModes(false),
		DefaultMode: agent.RunModeDefault,
	}
}
