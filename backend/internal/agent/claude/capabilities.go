package claude

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const capabilityTimeout = 8 * time.Second

var (
	quotedValuePattern = regexp.MustCompile(`['"]([A-Za-z0-9._-]+)['"]`)
	effortPattern      = regexp.MustCompile(`(?is)--effort\s+<level>.*?\(([^)]*)\)`)
	modePattern        = regexp.MustCompile(`(?is)--permission-mode\s+<mode>.*?choices:\s*([^)]+)\)`)
	modelPattern       = regexp.MustCompile(`(?is)--model\s+<model>.*?alias.*?\(([^)]*)\)`)
)

func (p *Provider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	probeCtx, cancel := context.WithTimeout(ctx, capabilityTimeout)
	defer cancel()
	cmd := agent.CapabilityCommand(
		probeCtx,
		req,
		[]string{"HOME=/root", "IS_SANDBOX=1"},
		"claude",
		"--help",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		caps := fallbackCapabilities()
		caps.Warning = "Claude capabilities could not be read from the CLI"
		return caps, fmt.Errorf("claude capability discovery: %w", err)
	}
	return parseCapabilities(string(output)), nil
}

func parseCapabilities(help string) agent.Capabilities {
	efforts := parseChoiceValues(effortPattern, help)
	models := parseQuotedValues(modelPattern, help)
	permissionModes := parseChoiceValues(modePattern, help)

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
		Modes:       agent.CodeAndPlanModes(containsFold(permissionModes, "plan")),
		DefaultMode: "code",
	}
}

func fallbackCapabilities() agent.Capabilities {
	reasoning := []agent.CapabilityOption{agent.AutoOption()}
	for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
		reasoning = append(reasoning, agent.CapabilityOption{Value: effort, Label: optionLabel(effort)})
	}
	models := make([]agent.ModelCapability, 0, 4)
	for _, id := range []string{"fable", "opus", "sonnet", "haiku"} {
		models = append(models, agent.ModelCapability{
			ID: id, Label: optionLabel(id), ReasoningEfforts: append([]agent.CapabilityOption(nil), reasoning...),
		})
	}
	return agent.Capabilities{
		Provider:    agent.ProviderClaude,
		Label:       "Claude",
		Source:      agent.CapabilitySourceFallback,
		Models:      agent.WithAutoModel(models, "Claude default"),
		Modes:       agent.CodeAndPlanModes(true),
		DefaultMode: "code",
	}
}

func parseChoiceValues(pattern *regexp.Regexp, input string) []string {
	match := pattern.FindStringSubmatch(input)
	if len(match) < 2 {
		return nil
	}
	parts := strings.FieldsFunc(match[1], func(r rune) bool {
		return r == ',' || r == '|' || unicode.IsSpace(r)
	})
	return uniqueSafeValues(parts)
}

func parseQuotedValues(pattern *regexp.Regexp, input string) []string {
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

func uniqueSafeValues(values []string) []string {
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

func containsFold(values []string, target string) bool {
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
