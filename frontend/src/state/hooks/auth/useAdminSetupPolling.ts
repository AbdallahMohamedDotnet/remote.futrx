import { useEffect } from "preact/hooks";

const adminSetupPollIntervalMs = 3_000;

export function useAdminSetupPolling(
  refresh: () => Promise<void>,
  enabled: boolean,
) {
  useEffect(() => {
    if (!enabled) return;

    let cancelled = false;
    let timer: number | undefined;
    const poll = async () => {
      try {
        await refresh();
      } finally {
        if (!cancelled) timer = window.setTimeout(poll, adminSetupPollIntervalMs);
      }
    };

    timer = window.setTimeout(poll, adminSetupPollIntervalMs);
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [enabled, refresh]);
}
