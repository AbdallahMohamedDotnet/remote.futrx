import type { UsageTotals } from "../../../state/chat/usage";
import { Activity } from "../../ui/icons";

export function UsagePill({
  totals,
  tokenLabel,
  costUsd,
}: {
  totals: UsageTotals;
  tokenLabel: string;
  costUsd: number;
}) {
  return (
    <div
      class="h-9 inline-flex items-center gap-2 px-3 rounded-md bg-white/5 border border-white/10
             text-[12.5px] text-ink-300 flex-none"
      title={`Input ${totals.inputTokens}\nOutput ${totals.outputTokens}\nCache read ${totals.cacheReadTokens}\nCache write ${totals.cacheWriteTokens}`}
    >
      <Activity class="w-4 h-4 text-accent-green" />
      <span>{tokenLabel} tokens</span>
      {costUsd > 0 && <span class="text-ink-100">${costUsd.toFixed(costUsd < 0.01 ? 4 : 2)}</span>}
    </div>
  );
}
