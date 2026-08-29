import type {
  UsageRange,
  UsageRangeLabels,
  UsageRangePreset,
} from "../models/usage.ts";

/**
 * Date-range selection for the Usage page. Days are bounded in UTC because the
 * ledger buckets records by UTC day, so a range picked in any timezone lines up
 * with the bars drawn from the same response.
 */
class UsageRangeService {
  private readonly dayMs = 24 * 60 * 60 * 1000;

  /** Start of the UTC day containing `at`. */
  startOfUtcDay(at: number): number {
    return Math.floor(at / this.dayMs) * this.dayMs;
  }

  /** Last millisecond of the UTC day containing `at`. */
  endOfUtcDay(at: number): number {
    return this.startOfUtcDay(at) + this.dayMs - 1;
  }

  /** ISO `YYYY-MM-DD` for the UTC day containing `at` — the `<input type=date>` value. */
  toDateInputValue(at: number): string {
    return new Date(this.startOfUtcDay(at)).toISOString().slice(0, 10);
  }

  /** Parses an ISO `YYYY-MM-DD` as a UTC day start, or null when malformed. */
  fromDateInputValue(value: string): number | null {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
    const parsed = Date.parse(`${value}T00:00:00.000Z`);
    return Number.isNaN(parsed) ? null : parsed;
  }

  /**
   * Resolves a preset against "now". `7d` and `30d` include today, so "7 days"
   * spans today plus the six days before it rather than a bare now-minus-7.
   */
  forPreset(preset: UsageRangePreset, now: number): UsageRange {
    const to = this.endOfUtcDay(now);
    switch (preset) {
      case "7d":
        return { preset, from: this.startOfUtcDay(now) - 6 * this.dayMs, to };
      case "month": {
        const today = new Date(this.startOfUtcDay(now));
        const from = Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), 1);
        return { preset, from, to };
      }
      case "custom":
      case "30d":
      default:
        return {
          preset: preset === "custom" ? "custom" : "30d",
          from: this.startOfUtcDay(now) - 29 * this.dayMs,
          to,
        };
    }
  }

  /**
   * Builds a custom range from two date inputs. Reversed inputs are swapped so
   * the picker cannot produce a window the backend rejects; malformed input
   * leaves the previous range untouched.
   */
  fromDates(current: UsageRange, fromValue: string, toValue: string): UsageRange {
    const from = this.fromDateInputValue(fromValue);
    const to = this.fromDateInputValue(toValue);
    if (from == null || to == null) return current;
    const [start, end] = from <= to ? [from, to] : [to, from];
    return { preset: "custom", from: start, to: this.endOfUtcDay(end) };
  }

  labels(range: UsageRange): UsageRangeLabels {
    return {
      fromDate: this.toDateInputValue(range.from),
      toDate: this.toDateInputValue(range.to),
    };
  }

  /** Number of whole UTC days the range covers, minimum one. */
  days(range: UsageRange): number {
    return Math.max(1, Math.round((range.to + 1 - range.from) / this.dayMs));
  }
}

export const usageRangeService = new UsageRangeService();
