package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	caps, err := parseCapabilities(modelsOutput, string(helpOutput))
	if err != nil {
		fallback := fallbackCapabilities()
		fallback.Warning = "Kimi returned an unreadable provider catalog"
		return fallback, err
	}
	return caps, nil
}

func parseCapabilities(raw []byte, help string) (agent.Capabilities, error) {
	modelIDs, err := parseModelIDs(raw)
	if err != nil {
		return agent.Capabilities{}, err
	}
	models := make([]agent.ModelCapability, 0, len(modelIDs))
	for _, id := range modelIDs {
		models = append(models, agent.ModelCapability{ID: id, Label: capabilityLabel(id)})
	}
	return agent.Capabilities{
		Provider:    agent.ProviderKimi,
		Label:       "Kimi",
		Source:      agent.CapabilitySourceLive,
		Models:      agent.WithAutoModel(models, "Kimi default"),
		Modes:       agent.CodeAndPlanModes(strings.Contains(help, "--plan")),
		DefaultMode: "code",
	}, nil
}

func parseModelIDs(raw []byte) ([]string, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var root struct {
		Models json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("decode kimi provider catalog: %w", err)
	}
	if len(root.Models) == 0 || string(root.Models) == "null" {
		return nil, nil
	}
	var byAlias map[string]json.RawMessage
	if err := json.Unmarshal(root.Models, &byAlias); err == nil {
		ids := make([]string, 0, len(byAlias))
		for alias := range byAlias {
			if safe := agent.NormalizeModelID(alias); safe != "" {
				ids = append(ids, safe)
			}
		}
		sort.Strings(ids)
		return ids, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(root.Models, &items); err != nil {
		return nil, fmt.Errorf("decode kimi models: %w", err)
	}
	seen := make(map[string]bool)
	ids := make([]string, 0, len(items))
	for _, item := range items {
		for _, field := range []string{"alias", "id", "model"} {
			value, _ := item[field].(string)
			value = agent.NormalizeModelID(value)
			if value != "" && !seen[value] {
				seen[value] = true
				ids = append(ids, value)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func fallbackCapabilities() agent.Capabilities {
	return agent.Capabilities{
		Provider:    agent.ProviderKimi,
		Label:       "Kimi",
		Source:      agent.CapabilitySourceFallback,
		Models:      agent.WithAutoModel(nil, "Kimi default"),
		Modes:       agent.CodeAndPlanModes(false),
		DefaultMode: "code",
	}
}

func capabilityLabel(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' })
	for index, part := range parts {
		if part != "" {
			parts[index] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	if len(parts) == 0 {
		return value
	}
	return strings.Join(parts, " ")
}
