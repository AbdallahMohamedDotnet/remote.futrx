import { useEffect, useState } from "preact/hooks";
import { chatQuestionCountdownService } from "../../../services/chat/chatQuestionCountdownService.ts";

export function useAutoResolutionCountdown(
  enabled: boolean,
  requestedAt?: number,
): number | null {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    setNow(Date.now());
    if (!enabled || requestedAt === undefined || !Number.isFinite(requestedAt)) return;

    const expiresAt = chatQuestionCountdownService.expiresAt(requestedAt);
    if (expiresAt <= Date.now()) return;
    const timer = window.setInterval(() => {
      const current = Date.now();
      setNow(current);
      if (current >= expiresAt) window.clearInterval(timer);
    }, 1_000);
    return () => window.clearInterval(timer);
  }, [enabled, requestedAt]);

  return enabled
    ? chatQuestionCountdownService.remainingSeconds(requestedAt, now)
    : null;
}
