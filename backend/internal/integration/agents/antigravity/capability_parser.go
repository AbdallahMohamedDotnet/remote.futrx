package antigravity

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

var (
	cliEffortChoicesPattern = regexp.MustCompile(`(?im)--effort\s+.*?\(([^)]*)\)`)
	cliModelLinePattern     = regexp.MustCompile(`^(\S+)[\t ]{2,}(.+)$`)
)

type cliModelDescriptor struct {
	ID    string
	Label string
}

type cliJSONModel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func parseCLIOutputCatalog(modelsOutput, help string) agent.Capabilities {
	efforts := parseCLIChoices(cliEffortChoicesPattern, help)
	reasoning := make([]agent.CapabilityOption, 0, len(efforts)+1)
	if len(efforts) > 0 {
		reasoning = append(reasoning, agent.AutoOption())
		for _, effort := range efforts {
			reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: capabilityLabel(effort)})
		}
	}
	descriptors := parseCLIModels(modelsOutput)
	models := make([]agent.ModelCapability, 0, len(descriptors))
	for _, model := range descriptors {
		models = append(models, agent.ModelCapability{
			ID: model.ID, Label: model.Label,
		})
	}
	models = agent.WithAutoModel(models, "Antigravity default")
	// agy's explicit model slugs already encode their effort variant. Combining
	// one of those slugs with --effort is not the same contract as selecting a
	// TUI model family, so expose the standalone --effort control only when agy
	// itself chooses the model.
	models[0].ReasoningEfforts = reasoning
	// The print transport cannot relay Antigravity's native approval lifecycle,
	// so --mode plan is not an end-to-end Remote capability yet.
	capabilities := agent.Capabilities{
		Provider:    agent.ProviderAntigravity,
		Label:       "Antigravity",
		Source:      agent.CapabilitySourceLive,
		Models:      models,
		Modes:       agent.ProviderModes(false),
		DefaultMode: agent.RunModeDefault,
	}
	if len(descriptors) == 0 {
		capabilities.Source = agent.CapabilitySourceFallback
		capabilities.Warning = "Antigravity returned no usable model catalog"
	}
	return capabilities
}

func parseCLIModels(output string) []cliModelDescriptor {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}
	if models, ok := parseCLIJSONModels(trimmed); ok {
		return uniqueCLIModels(models)
	}

	models := make([]cliModelDescriptor, 0)
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if line == "" || strings.Contains(lower, "sign in") || strings.Contains(lower, "available model") ||
			strings.HasPrefix(lower, "fetching ") ||
			strings.HasPrefix(lower, "usage") || strings.HasPrefix(lower, "flags") ||
			strings.EqualFold(line, "model") || strings.HasPrefix(line, "---") {
			continue
		}
		line = strings.TrimLeft(line, "*-•>✓ ")
		match := cliModelLinePattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		id := normalizeCLIModelID(match[1])
		label := normalizeCLIModelLabel(match[2])
		if id != "" && label != "" {
			models = append(models, cliModelDescriptor{ID: id, Label: label})
		}
	}
	return uniqueCLIModels(models)
}

func parseCLIJSONModels(input string) ([]cliModelDescriptor, bool) {
	var models []cliJSONModel
	switch {
	case strings.HasPrefix(input, "{"):
		var root struct {
			Command struct {
				Name string `json:"name"`
				Data struct {
					Models []cliJSONModel `json:"models"`
				} `json:"data"`
			} `json:"command"`
		}
		if json.Unmarshal([]byte(input), &root) != nil {
			return nil, true
		}
		if root.Command.Name != "models" {
			return nil, true
		}
		models = root.Command.Data.Models
	default:
		return nil, false
	}

	result := make([]cliModelDescriptor, 0, len(models))
	for _, model := range models {
		// The label is presentation data and is not accepted by --model. The
		// catalog ID is the stable slug that agy accepts through --model.
		id := normalizeCLIModelID(model.ID)
		if id == "" {
			continue
		}
		label := normalizeCLIModelLabel(model.Label)
		if label == "" {
			label = id
		}
		result = append(result, cliModelDescriptor{ID: id, Label: label})
	}
	return result, true
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

func uniqueCLIModels(values []cliModelDescriptor) []cliModelDescriptor {
	seen := make(map[string]bool)
	result := make([]cliModelDescriptor, 0, len(values))
	for _, value := range values {
		value.ID = normalizeCLIModelID(value.ID)
		value.Label = normalizeCLIModelLabel(value.Label)
		if value.ID == "" || value.Label == "" || seen[value.ID] {
			continue
		}
		seen[value.ID] = true
		result = append(result, value)
	}
	return result
}

func normalizeCLIModelID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 {
		return ""
	}
	return agent.NormalizeModelID(value)
}

func normalizeCLIModelLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 {
		return ""
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return value
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
