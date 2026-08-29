import { useCallback, useState } from "preact/hooks";
import type { UsageRange, UsageRangePreset } from "../../../models/usage";
import { usageRangeService } from "../../../services/usageRangeService.ts";

/** The selected window. Owns nothing but the range and how it is chosen. */
export function useUsageRange() {
  const [range, setRange] = useState<UsageRange>(() => usageRangeService.forPreset("30d", Date.now()));

  const setPreset = useCallback((preset: UsageRangePreset) => {
    setRange((current) =>
      preset === "custom" ? { ...current, preset } : usageRangeService.forPreset(preset, Date.now())
    );
  }, []);

  const setCustomRange = useCallback((from: string, to: string) => {
    setRange((current) => usageRangeService.fromDates(current, from, to));
  }, []);

  return { range, setPreset, setCustomRange };
}
