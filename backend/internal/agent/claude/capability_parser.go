package claude

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

var (
	quotedValuePattern = regexp.MustCompile(`['"]([A-Za-z0-9._-]+)['"]`)
	effortPattern      = regexp.MustCompile(`(?is)--effort\s+<level>.*?\(([^)]*)\)`)
	modePattern        = regexp.MustCompile(`(?is)--permission-mode\s+<mode>.*?choices:\s*([^)]+)\)`)
	modelPattern       = regexp.MustCompile(`(?is)--model\s+<model>.*?alias.*?\(([^)]*)\)`)
)

func parseCapabilityHelp(help string) agent.Capabilities {
	efforts := parseHelpChoiceValues(effortPattern, help)
	models := parseHelpQuotedValues(modelPattern, help)
	permissionModes := parseHelpChoiceValues(modePattern, help)

	if len(efforts) == 0 || len(models) == 0 {
		caps := fallbackCapabilities()
		caps.Warning = "Claude CLI help did not publish a complete capability catalog"
		return caps
	}

	reasoning := []agent.CapabilityOption{agent.AutoOption()}
	for _, effort := range efforts {
		reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: optionLabel(effort)})
	}
	items := make([]agent.ModelCapability, 0, len(models))
	for _, id := range models {
		items = append(items, agent.ModelCapability{
			ID: id, Label: optionLabel(id), ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		})
	}
	return agent.Capabilities{
		Provider:    agent.ProviderClaude,
		Label:       "Claude",
		Source:      agent.CapabilitySourceLive,
		Models:      agent.WithAutoModel(items, "Claude default"),
		Modes:       agent.ProviderModes(containsChoice(permissionModes, "plan")),
		DefaultMode: "default",
	}
}

func parseHelpChoiceValues(pattern *regexp.Regexp, input string) []string {
	match := pattern.FindStringSubmatch(input)
	if len(match) < 2 {
		return nil
	}
	parts := strings.FieldsFunc(match[1], func(r rune) bool {
		return r == ',' || r == '|' || unicode.IsSpace(r)
	})
	return uniqueHelpValues(parts)
}

func parseHelpQuotedValues(pattern *regexp.Regexp, input string) []string {
	match := pattern.FindStringSubmatch(input)
	if len(match) < 2 {
		return nil
	}
	quoted := quotedValuePattern.FindAllStringSubmatch(match[1], -1)
	seen := make(map[string]bool)
	values := make([]string, 0, len(quoted))
	for _, item := range quoted {
		if len(item) > 1 {
			value := agent.NormalizeModelID(item[1])
			if value != "" && !seen[value] {
				seen[value] = true
				values = append(values, value)
			}
		}
	}
	return values
}

func uniqueHelpValues(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = agent.NormalizeCapabilityValue(strings.Trim(value, "'\""))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func containsChoice(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func optionLabel(value string) string {
	if strings.EqualFold(value, "xhigh") {
		return "XHigh"
	}
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
