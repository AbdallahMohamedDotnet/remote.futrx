import { useCallback, useState } from "preact/hooks";
import {
  usageRangeForPreset,
  usageRangeFromDates,
  type UsageRange,
  type UsageRangePreset,
} from "../../usage/usageRangeState";

/** The selected window. Owns nothing but the range and how it is chosen. */
export function useUsageRange() {
  const [range, setRange] = useState<UsageRange>(() => usageRangeForPreset("30d", Date.now()));

  const setPreset = useCallback((preset: UsageRangePreset) => {
    setRange((current) =>
      preset === "custom" ? { ...current, preset } : usageRangeForPreset(preset, Date.now())
    );
  }, []);

  const setCustomRange = useCallback((from: string, to: string) => {
    setRange((current) => usageRangeFromDates(current, from, to));
  }, []);

  return { range, setPreset, setCustomRange };
}
