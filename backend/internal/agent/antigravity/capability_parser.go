package antigravity

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

var (
	cliEffortChoicesPattern = regexp.MustCompile(`(?im)--effort\s+.*?\(([^)]*)\)`)
	cliModeChoicesPattern   = regexp.MustCompile(`(?im)--mode\s+.*?\(([^)]*)\)`)
	cliParenthesizedID      = regexp.MustCompile(`\(([A-Za-z0-9._:/@-]+)\)\s*$`)
)

func parseCLIOutputCatalog(modelsOutput, help string) agent.Capabilities {
	efforts := parseCLIChoices(cliEffortChoicesPattern, help)
	reasoning := make([]agent.CapabilityOption, 0, len(efforts)+1)
	if len(efforts) > 0 {
		reasoning = append(reasoning, agent.AutoOption())
		for _, effort := range efforts {
			reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: capabilityLabel(effort)})
		}
	}
	modelIDs := parseCLIModelIDs(modelsOutput)
	models := make([]agent.ModelCapability, 0, len(modelIDs))
	for _, id := range modelIDs {
		models = append(models, agent.ModelCapability{
			ID: id, Label: capabilityLabel(id), ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		})
	}
	modes := parseCLIChoices(cliModeChoicesPattern, help)
	return agent.Capabilities{
		Provider:    agent.ProviderAntigravity,
		Label:       "Antigravity",
		Source:      agent.CapabilitySourceLive,
		Models:      agent.WithAutoModel(models, "Antigravity default"),
		Modes:       agent.CapabilityModes(containsCLIChoice(modes, "plan")),
		DefaultMode: "code",
	}
}

func parseCLIModelIDs(output string) []string {
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
			id := firstCLIModelID(model.ID, model.Model, model.Name)
			if id != "" {
				ids = append(ids, id)
			}
		}
		return uniqueCLIModels(ids)
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
		if match := cliParenthesizedID.FindStringSubmatch(line); len(match) > 1 {
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
	return uniqueCLIModels(ids)
}

func parseCLIChoices(pattern *regexp.Regexp, input string) []string {
	match := pattern.FindStringSubmatch(input)
	if len(match) < 2 {
		return nil
	}
	return uniqueCLIValues(strings.FieldsFunc(match[1], func(r rune) bool {
		return r == '|' || r == ',' || r == ' '
	}))
}

func uniqueCLIValues(values []string) []string {
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

func uniqueCLIModels(values []string) []string {
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

func firstCLIModelID(values ...string) string {
	for _, value := range values {
		if safe := agent.NormalizeModelID(value); safe != "" {
			return safe
		}
	}
	return ""
}

func containsCLIChoice(values []string, target string) bool {
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
