package kimi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func parseProviderCatalog(raw []byte, help string) (agent.Capabilities, error) {
	modelIDs, err := parseProviderModelIDs(raw)
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

func parseProviderModelIDs(raw []byte) ([]string, error) {
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
