import {
  CODEX_AUTO_RESOLUTION_VISIBLE_MS,
  CODEX_NON_BLOCKING_AUTO_RESOLUTION_MS,
} from "../../config/chat.ts";

class ChatQuestionCountdownService {
  expiresAt(requestedAt: number): number {
    return requestedAt + CODEX_NON_BLOCKING_AUTO_RESOLUTION_MS;
  }

  remainingSeconds(requestedAt: number | undefined, now: number): number | null {
    if (requestedAt === undefined || !Number.isFinite(requestedAt) || !Number.isFinite(now)) {
      return null;
    }

    const remaining = this.expiresAt(requestedAt) - now;
    if (remaining <= 0 || remaining > CODEX_AUTO_RESOLUTION_VISIBLE_MS) {
      return null;
    }
    return Math.max(1, Math.ceil(remaining / 1_000));
  }

  formatRemaining(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;
    return `${Math.floor(seconds / 60)}m ${String(seconds % 60).padStart(2, "0")}s`;
  }
}

export const chatQuestionCountdownService = new ChatQuestionCountdownService();
