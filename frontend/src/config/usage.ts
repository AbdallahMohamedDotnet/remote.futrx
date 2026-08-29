import type { UsageRangePreset } from "../models/usage";

/** The windows the Usage page offers, in the order it lists them. */
export const USAGE_RANGE_PRESETS: Array<{ id: UsageRangePreset; label: string }> = [
  { id: "7d", label: "7 days" },
  { id: "30d", label: "30 days" },
  { id: "month", label: "This month" },
  { id: "custom", label: "Custom" },
];
