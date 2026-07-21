import type { ChatMode, ChatProvider, ReasoningEffort, ServiceTier } from "../models/chat";

export const PROVIDER_OPTIONS: Array<{ value: ChatProvider; label: string }> = [
  { value: "codex", label: "Codex" },
  { value: "claude", label: "Claude" },
  { value: "kimi", label: "Kimi" },
];

const CLAUDE_MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "", label: "Auto", sub: "server default" },
  { value: "fable", label: "Fable", sub: "most capable" },
  { value: "opus", label: "Opus", sub: "deepest reasoning" },
  { value: "sonnet", label: "Sonnet", sub: "balanced" },
  { value: "haiku", label: "Haiku", sub: "fast" },
];

const CODEX_MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "", label: "Auto", sub: "codex default" },
  { value: "gpt-5.6-sol", label: "GPT-5.6 Sol", sub: "flagship preview" },
  { value: "gpt-5.5", label: "GPT-5.5", sub: "frontier coding" },
  { value: "gpt-5.4", label: "GPT-5.4", sub: "strong everyday coding" },
  { value: "gpt-5.4-mini", label: "GPT-5.4 Mini", sub: "fast" },
  { value: "gpt-5.3-codex", label: "GPT-5.3 Codex", sub: "coding optimized" },
];

const KIMI_MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "", label: "Auto", sub: "kimi default" },
];

export function modelOptionsForProvider(provider?: ChatProvider) {
  if (provider === "codex") return CODEX_MODEL_OPTIONS;
  if (provider === "kimi") return KIMI_MODEL_OPTIONS;
  return CLAUDE_MODEL_OPTIONS;
}

export const MODE_OPTIONS: Array<{ value: ChatMode; label: string }> = [
  { value: "chat", label: "Chat" },
  { value: "plan", label: "Plan" },
  { value: "code", label: "Code" },
  { value: "review", label: "Review" },
  { value: "debug", label: "Debug" },
  { value: "full-auto", label: "Full auto" },
];

type ReasoningEffortOption = { value: ReasoningEffort; label: string };

// Reasoning-effort ladders differ per CLI (verified against each provider's
// --help / config validation):
//   Claude `claude --effort`:            low, medium, high, xhigh, max, ultra
//   Codex  `-c model_reasoning_effort=`: none, minimal, low, medium, high, xhigh, max, ultra
//   Kimi:  no reasoning/effort flag at all
// "Auto" ("") omits the flag so the CLI/server picks its own default.
const CLAUDE_REASONING_EFFORT_OPTIONS: ReasoningEffortOption[] = [
  { value: "", label: "Auto" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "xhigh", label: "XHigh" },
  { value: "max", label: "Max" },
  { value: "ultra", label: "Ultra" },
];

const CODEX_REASONING_EFFORT_OPTIONS: ReasoningEffortOption[] = [
  { value: "", label: "Auto" },
  { value: "none", label: "None" },
  { value: "minimal", label: "Minimal" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "xhigh", label: "XHigh" },
  { value: "max", label: "Max" },
  { value: "ultra", label: "Ultra" },
];

export function reasoningEffortOptionsForProvider(
  provider?: ChatProvider,
): ReasoningEffortOption[] {
  if (provider === "claude") return CLAUDE_REASONING_EFFORT_OPTIONS;
  if (provider === "kimi") return [];
  return CODEX_REASONING_EFFORT_OPTIONS;
}

type ServiceTierOption = { value: ServiceTier; label: string };

// Codex `service_tier` is the only headless speed lever across our providers
// (Claude's fast mode is interactive-only; Kimi has none). Values are
// model-gated — the flagship advertises default/priority and unsupported tiers
// are warned-and-omitted by the CLI. "Auto" ("") omits the flag entirely.
const CODEX_SERVICE_TIER_OPTIONS: ServiceTierOption[] = [
  { value: "", label: "Auto" },
  { value: "default", label: "Default" },
  { value: "priority", label: "Priority" },
  { value: "fast", label: "Fast" },
];

export function serviceTierOptionsForProvider(
  provider?: ChatProvider,
): ServiceTierOption[] {
  if (provider === "codex") return CODEX_SERVICE_TIER_OPTIONS;
  return [];
}

export function providerDisplayLabel(provider?: ChatProvider): string {
  if (provider === "codex") return "Codex";
  if (provider === "kimi") return "Kimi";
  return "Claude";
}

export function modelDisplayLabel(model?: string, provider?: ChatProvider): string {
  if (!model) return "Auto";
  const lower = model.toLowerCase();
  if (provider === "codex") {
    const match = CODEX_MODEL_OPTIONS.find((option) => option.value === model);
    return match?.label ?? model;
  }
  if (lower.includes("fable")) return "Fable";
  if (lower.includes("opus")) return "Opus";
  if (lower.includes("sonnet")) return "Sonnet";
  if (lower.includes("haiku")) return "Haiku";
  return model;
}
