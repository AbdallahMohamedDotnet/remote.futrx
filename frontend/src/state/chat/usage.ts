import type { ChatEvent } from "../../models/chat";

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
