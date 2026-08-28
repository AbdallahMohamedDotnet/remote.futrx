import { useEffect, useState } from "preact/hooks";
import {
  autoResolutionRemainingSeconds,
  CODEX_NON_BLOCKING_AUTO_RESOLUTION_MS,
} from "./autoResolutionCountdown";

export function useAutoResolutionCountdown(
  enabled: boolean,
  requestedAt?: number,
): number | null {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    setNow(Date.now());
    if (!enabled || requestedAt === undefined || !Number.isFinite(requestedAt)) return;

    const expiresAt = requestedAt + CODEX_NON_BLOCKING_AUTO_RESOLUTION_MS;
    if (expiresAt <= Date.now()) return;
    const timer = window.setInterval(() => {
      const current = Date.now();
      setNow(current);
      if (current >= expiresAt) window.clearInterval(timer);
    }, 1_000);
    return () => window.clearInterval(timer);
  }, [enabled, requestedAt]);

  return enabled ? autoResolutionRemainingSeconds(requestedAt, now) : null;
}
