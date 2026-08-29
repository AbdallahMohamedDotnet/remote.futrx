import { useCallback, useEffect, useState } from "preact/hooks";
import { usageApi } from "../../../api/usageApi";
import type { UsageRecord } from "../../../models/usage";
import type { UsageRange } from "../../logic/usage/usageRangeState";

const DRILL_DOWN_LIMIT = 100;

export interface UsageDrillDown {
  projectId: string;
  label: string;
  records: UsageRecord[];
  loading: boolean;
  error: string | null;
  hasMore: boolean;
}

/** The per-project record list opened from a summary row, paged by cursor. */
export function useUsageDrillDown(range: UsageRange) {
  const [drillDown, setDrillDown] = useState<UsageDrillDown | null>(null);
  const [cursor, setCursor] = useState<string | undefined>(undefined);

  // A new window invalidates whatever rows the drill-down was showing.
  useEffect(() => {
    setDrillDown(null);
    setCursor(undefined);
  }, [range.from, range.to]);

  const fetchRecords = useCallback(
    async (projectId: string, label: string, nextCursor?: string) => {
      setDrillDown((current) => ({
        projectId,
        label,
        records: nextCursor && current ? current.records : [],
        loading: true,
        error: null,
        hasMore: false,
      }));
      try {
        const page = await usageApi.records({
          from: range.from,
          to: range.to,
          projectId: projectId || undefined,
          limit: DRILL_DOWN_LIMIT,
          cursor: nextCursor,
        });
        setCursor(page.nextCursor);
        setDrillDown((current) => ({
          projectId,
          label,
          records: nextCursor && current ? [...current.records, ...page.records] : page.records,
          loading: false,
          error: null,
          hasMore: Boolean(page.nextCursor),
        }));
      } catch (cause) {
        setDrillDown((current) => ({
          projectId,
          label,
          records: current?.records ?? [],
          loading: false,
          error: (cause as Error).message,
          hasMore: false,
        }));
      }
    },
    [range.from, range.to]
  );

  const openDrillDown = useCallback(
    async (projectId: string, label: string) => {
      setCursor(undefined);
      await fetchRecords(projectId, label);
    },
    [fetchRecords]
  );

  const loadMoreDrillDown = useCallback(async () => {
    if (!drillDown || !cursor) return;
    await fetchRecords(drillDown.projectId, drillDown.label, cursor);
  }, [drillDown, cursor, fetchRecords]);

  const closeDrillDown = useCallback(() => setDrillDown(null), []);

  return { drillDown, openDrillDown, loadMoreDrillDown, closeDrillDown };
}
