import type { ChatEvent } from "../../models/chat";
import type { ChatMode } from "../../models/chat";
import type { ChatProvider } from "../../models/chat";
import type { ReasoningEffort } from "../../models/chat";

export const PROVIDER_OPTIONS: Array<{ value: ChatProvider; label: string }> = [
  { value: "codex", label: "Codex" },
  { value: "claude", label: "Claude" },
  { value: "kimi", label: "Kimi" },
];

export const CLAUDE_MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "", label: "Auto", sub: "server default" },
  { value: "opus", label: "Opus", sub: "deepest reasoning" },
  { value: "sonnet", label: "Sonnet", sub: "balanced" },
  { value: "haiku", label: "Haiku", sub: "fast" },
];

export const CODEX_MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "", label: "Auto", sub: "codex default" },
  { value: "gpt-5.5", label: "GPT-5.5", sub: "frontier coding" },
  { value: "gpt-5.4", label: "GPT-5.4", sub: "strong everyday coding" },
  { value: "gpt-5.4-mini", label: "GPT-5.4 Mini", sub: "fast" },
  { value: "gpt-5.3-codex", label: "GPT-5.3 Codex", sub: "coding optimized" },
];

export const KIMI_MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "", label: "Auto", sub: "kimi default" },
];

export const MODEL_OPTIONS = CLAUDE_MODEL_OPTIONS;

export function modelOptionsForProvider(provider?: ChatProvider) {
  if (provider === "codex") return CODEX_MODEL_OPTIONS;
  if (provider === "kimi") return KIMI_MODEL_OPTIONS;
  return CLAUDE_MODEL_OPTIONS;
}

export function composerModelOptionsForProvider(provider?: ChatProvider) {
  return modelOptionsForProvider(provider).map(({ value, label }) => ({ value, label }));
}

export const MODE_OPTIONS: Array<{ value: ChatMode; label: string }> = [
  { value: "chat", label: "Chat" },
  { value: "plan", label: "Plan" },
  { value: "code", label: "Code" },
  { value: "review", label: "Review" },
  { value: "debug", label: "Debug" },
  { value: "full-auto", label: "Full auto" },
];

export const REASONING_EFFORT_OPTIONS: Array<{ value: ReasoningEffort; label: string }> = [
  { value: "", label: "Auto" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "xhigh", label: "XHigh" },
];

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
  if (lower.includes("opus")) return "Opus";
  if (lower.includes("sonnet")) return "Sonnet";
  if (lower.includes("haiku")) return "Haiku";
  return model;
}

export interface UsageTotals {
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
}

export const EMPTY_USAGE_TOTALS: UsageTotals = {
  inputTokens: 0,
  outputTokens: 0,
  cacheReadTokens: 0,
  cacheWriteTokens: 0,
};

type Usage =
  | {
      input_tokens?: number;
      output_tokens?: number;
      cache_read_input_tokens?: number;
      cache_creation_input_tokens?: number;
    }
  | null;

const COST = {
  opus: { in: 15.0, out: 75.0, cacheRead: 1.5, cacheWrite: 18.75 },
  sonnet: { in: 3.0, out: 15.0, cacheRead: 0.3, cacheWrite: 3.75 },
  haiku: { in: 0.8, out: 4.0, cacheRead: 0.08, cacheWrite: 1.0 },
};

export function computeUsageTotals(events: ChatEvent[]): UsageTotals {
  let totals = EMPTY_USAGE_TOTALS;

  for (const event of events) {
    totals = addUsageFromEvent(totals, event);
  }

  return totals;
}

export function addUsageFromEvent(totals: UsageTotals, event: ChatEvent): UsageTotals {
  if (event.type !== "complete" || !event.usage) return totals;

  try {
    const usage = (typeof event.usage === "string" ? JSON.parse(event.usage) : event.usage) as Usage;
    if (!usage) return totals;
    return {
      inputTokens: totals.inputTokens + (usage.input_tokens ?? 0),
      outputTokens: totals.outputTokens + (usage.output_tokens ?? 0),
      cacheReadTokens: totals.cacheReadTokens + (usage.cache_read_input_tokens ?? 0),
      cacheWriteTokens: totals.cacheWriteTokens + (usage.cache_creation_input_tokens ?? 0),
    };
  } catch {
    return totals;
  }
}

export function tokenTotal(totals: UsageTotals): number {
  return totals.inputTokens + totals.outputTokens + totals.cacheReadTokens + totals.cacheWriteTokens;
}

export function estimateCost(totals: UsageTotals, model: string): number {
  const key = modelDisplayLabel(model).toLowerCase() as keyof typeof COST;
  const cost = COST[key] ?? COST.sonnet;
  return (
    (totals.inputTokens * cost.in +
      totals.outputTokens * cost.out +
      totals.cacheReadTokens * cost.cacheRead +
      totals.cacheWriteTokens * cost.cacheWrite) /
    1_000_000
  );
}

export function formatTokens(n?: number): string {
  if (!n && n !== 0) return "0";
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(n >= 10_000 ? 0 : 1) + "k";
  return String(n);
}
