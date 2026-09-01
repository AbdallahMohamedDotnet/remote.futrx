import { chatQuestionCountdownService } from "../../../../services/chat/chatQuestionCountdownService.ts";

export {
  CODEX_AUTO_RESOLUTION_VISIBLE_MS,
  CODEX_NON_BLOCKING_AUTO_RESOLUTION_MS,
} from "../../../../config/chat.ts";

export function autoResolutionRemainingSeconds(
  requestedAt: number | undefined,
  now: number,
): number | null {
  return chatQuestionCountdownService.remainingSeconds(requestedAt, now);
}

export function formatAutoResolutionRemaining(seconds: number): string {
  return chatQuestionCountdownService.formatRemaining(seconds);
}
