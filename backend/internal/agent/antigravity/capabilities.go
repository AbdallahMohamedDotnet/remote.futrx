package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const capabilityTimeout = 10 * time.Second

var (
	effortChoicesPattern = regexp.MustCompile(`(?im)--effort\s+.*?\(([^)]*)\)`)
	modeChoicesPattern   = regexp.MustCompile(`(?im)--mode\s+.*?\(([^)]*)\)`)
	parenthesizedID      = regexp.MustCompile(`\(([A-Za-z0-9._:/@-]+)\)\s*$`)
)

func (p *Provider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	probeCtx, cancel := context.WithTimeout(ctx, capabilityTimeout)
	defer cancel()
	environment := []string{"HOME=" + containerAgentHome}

	modelsCmd := agent.CapabilityCommand(probeCtx, req, environment, "agy", "models")
	modelsOutput, modelsErr := modelsCmd.CombinedOutput()
	helpCmd := agent.CapabilityCommand(probeCtx, req, environment, "agy", "--help")
	helpOutput, helpErr := helpCmd.CombinedOutput()

	if modelsErr != nil && helpErr != nil {
		caps := fallbackCapabilities()
		caps.Warning = "Antigravity capabilities could not be read from the CLI"
		return caps, fmt.Errorf("antigravity capability discovery: models: %v; help: %w", modelsErr, helpErr)
	}
	caps := parseCapabilities(string(modelsOutput), string(helpOutput))
	if modelsErr != nil {
		caps.Source = agent.CapabilitySourceFallback
		caps.Warning = "Sign in to Antigravity in this project to load its model catalog"
		return caps, modelsErr
	}
	return caps, nil
}

func parseCapabilities(modelsOutput, help string) agent.Capabilities {
	efforts := parsePipeChoices(effortChoicesPattern, help)
	reasoning := make([]agent.CapabilityOption, 0, len(efforts)+1)
	if len(efforts) > 0 {
		reasoning = append(reasoning, agent.AutoOption())
		for _, effort := range efforts {
			reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: capabilityLabel(effort)})
		}
	}
	modelIDs := parseModelIDs(modelsOutput)
	models := make([]agent.ModelCapability, 0, len(modelIDs))
	for _, id := range modelIDs {
		models = append(models, agent.ModelCapability{
			ID: id, Label: capabilityLabel(id), ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		})
	}
	modes := parsePipeChoices(modeChoicesPattern, help)
	return agent.Capabilities{
		Provider:    agent.ProviderAntigravity,
		Label:       "Antigravity",
		Source:      agent.CapabilitySourceLive,
		Models:      agent.WithAutoModel(models, "Antigravity default"),
		Modes:       agent.CodeAndPlanModes(containsFold(modes, "plan")),
		DefaultMode: "code",
	}
}

func parseModelIDs(output string) []string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}
	var jsonObject struct {
		Models []struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Name  string `json:"name"`
		} `json:"models"`
	}
	if strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &jsonObject) == nil {
		ids := make([]string, 0, len(jsonObject.Models))
		for _, model := range jsonObject.Models {
			id := firstSafe(model.ID, model.Model, model.Name)
			if id != "" {
				ids = append(ids, id)
			}
		}
		return uniqueSortedModels(ids)
	}

	ids := make([]string, 0)
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if line == "" || strings.Contains(lower, "sign in") || strings.Contains(lower, "available model") ||
			strings.HasPrefix(lower, "usage") || strings.HasPrefix(lower, "flags") ||
			strings.EqualFold(line, "model") || strings.HasPrefix(line, "---") {
			continue
		}
		if match := parenthesizedID.FindStringSubmatch(line); len(match) > 1 {
			if id := agent.NormalizeModelID(match[1]); id != "" {
				ids = append(ids, id)
				continue
			}
		}
		line = strings.TrimLeft(line, "*-•>✓ ")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if id := agent.NormalizeModelID(fields[0]); id != "" && strings.ContainsAny(id, "-._/:@") {
			ids = append(ids, id)
		}
	}
	return uniqueSortedModels(ids)
}

func parsePipeChoices(pattern *regexp.Regexp, input string) []string {
	match := pattern.FindStringSubmatch(input)
	if len(match) < 2 {
		return nil
	}
	return uniqueValues(strings.FieldsFunc(match[1], func(r rune) bool {
		return r == '|' || r == ',' || r == ' '
	}))
}

func uniqueValues(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = agent.NormalizeCapabilityValue(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func uniqueSortedModels(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = agent.NormalizeModelID(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
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

func firstSafe(values ...string) string {
	for _, value := range values {
		if safe := agent.NormalizeModelID(value); safe != "" {
			return safe
		}
	}
	return ""
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
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
