import type { ChatEvent } from "../../models/chat";
import { modelDisplayLabel } from "../../config/chat";

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
