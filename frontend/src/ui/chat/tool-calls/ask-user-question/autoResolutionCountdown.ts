export const CODEX_NON_BLOCKING_AUTO_RESOLUTION_MS = 120_000;
export const CODEX_AUTO_RESOLUTION_VISIBLE_MS = 60_000;

export function autoResolutionRemainingSeconds(
  requestedAt: number | undefined,
  now: number,
): number | null {
  if (requestedAt === undefined || !Number.isFinite(requestedAt) || !Number.isFinite(now)) {
    return null;
  }

  const remaining = requestedAt + CODEX_NON_BLOCKING_AUTO_RESOLUTION_MS - now;
  if (remaining <= 0 || remaining > CODEX_AUTO_RESOLUTION_VISIBLE_MS) {
    return null;
  }
  return Math.max(1, Math.ceil(remaining / 1_000));
}

export function formatAutoResolutionRemaining(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  return `${Math.floor(seconds / 60)}m ${String(seconds % 60).padStart(2, "0")}s`;
}
