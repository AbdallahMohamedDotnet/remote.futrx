package agent

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

// CapabilityCommand runs a provider probe either on the host or inside the
// selected project container. Provider adapters supply only the environment
// variables their CLI needs for capability discovery.
func CapabilityCommand(
	ctx context.Context,
	req CapabilityRequest,
	environment []string,
	binary string,
	args ...string,
) *exec.Cmd {
	if strings.TrimSpace(req.ContainerName) == "" {
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = mergeEnvironment(os.Environ(), environment)
		return cmd
	}
	lxcArgs := []string{"exec", "--cwd", "/workspace"}
	for _, entry := range environment {
		lxcArgs = append(lxcArgs, "--env", entry)
	}
	lxcArgs = append(lxcArgs, req.ContainerName, "--", binary)
	lxcArgs = append(lxcArgs, args...)
	return exec.CommandContext(ctx, "lxc", lxcArgs...)
}

func mergeEnvironment(base, overrides []string) []string {
	overridden := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		name, _, found := strings.Cut(entry, "=")
		if found {
			overridden[name] = struct{}{}
		}
	}

	merged := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, replaced := overridden[name]; replaced {
				continue
			}
		}
		merged = append(merged, entry)
	}
	return append(merged, overrides...)
}

// NormalizeCapabilityValue accepts future provider values without embedding a
// moving enum in Remote while still rejecting strings that are unsafe to place
// inside provider configuration arguments.
func NormalizeCapabilityValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		return ""
	}
	return value
}

// NormalizeModelID is deliberately less restrictive than capability values:
// custom provider registries commonly use namespaced ids such as org/model or
// provider:model. Model ids are always passed as a distinct process argument.
func NormalizeModelID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_.:/@", r) {
			continue
		}
		return ""
	}
	return value
}

func AutoOption() CapabilityOption {
	return CapabilityOption{Value: "", Label: "Auto"}
}

func CodeAndPlanModes(nativePlan bool) []CapabilityOption {
	return []CapabilityOption{
		{Value: "code", Label: "Code", Description: "Use the provider's default coding mode", Native: true},
		{Value: "plan", Label: "Plan", Description: "Inspect and plan before making changes", Native: nativePlan},
		{Value: "chat", Label: "Chat", Description: "Answer without changing files unless asked"},
		{Value: "review", Label: "Review", Description: "Prioritize bugs, regressions, tests, and risks"},
		{Value: "debug", Label: "Debug", Description: "Reproduce and localize the issue before fixing it"},
		{Value: "full-auto", Label: "Full auto", Description: "Carry implementation through verification"},
	}
}

// WithAutoModel prepends the provider-default selection. Its controls mirror
// the provider's declared default model so dependent selectors remain valid
// while the CLI chooses the actual model.
func WithAutoModel(models []ModelCapability, description string) []ModelCapability {
	auto := ModelCapability{ID: "", Label: "Auto", Description: description}
	for _, model := range models {
		if model.ProviderDefault {
			auto.ReasoningEfforts = cloneOptions(model.ReasoningEfforts)
			auto.DefaultReasoningEffort = model.DefaultReasoningEffort
			auto.ServiceTiers = cloneOptions(model.ServiceTiers)
			auto.DefaultServiceTier = model.DefaultServiceTier
			break
		}
	}
	if len(auto.ReasoningEfforts) == 0 && len(models) > 0 {
		auto.ReasoningEfforts = cloneOptions(models[0].ReasoningEfforts)
		auto.DefaultReasoningEffort = models[0].DefaultReasoningEffort
		auto.ServiceTiers = cloneOptions(models[0].ServiceTiers)
		auto.DefaultServiceTier = models[0].DefaultServiceTier
	}
	return append([]ModelCapability{auto}, models...)
}

func cloneOptions(options []CapabilityOption) []CapabilityOption {
	return append([]CapabilityOption(nil), options...)
}
