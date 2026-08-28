import type { ChatStatus, PromptOutcome, QueuedPrompt } from "../../models/chat";

// Delivery policy for queued prompts. A queued prompt is only removed once the
// server acknowledges that a run accepted it. Retryable busy rejections stay
// queued; semantic rejections are removed for explicit user review so they can
// never auto-run later under different execution preferences.
class PromptQueueState {
  // The prompt to put on the wire now, or null if the window is closed, a
  // dispatch is already in flight, or the queue is empty.
  nextDispatch(
    prompts: QueuedPrompt[],
    inflightId: string | null,
    status: ChatStatus,
    canSendPrompt: boolean,
  ): QueuedPrompt | null {
    if (status !== "ready" || !canSendPrompt) return null;
    if (inflightId !== null) return null;
    return prompts[0] ?? null;
  }

  // Accepted prompts now live in the transcript. Non-retryable rejections are
  // also removed and restored to the composer by the hook; only a busy verdict
  // remains eligible for automatic retry.
  promptsAfterOutcome(prompts: QueuedPrompt[], outcome: PromptOutcome): QueuedPrompt[] {
    if (!prompts.some((prompt) => prompt.id === outcome.clientId)) return prompts;
    if (!outcome.accepted && outcome.retryable) return prompts;
    return prompts.filter((prompt) => prompt.id !== outcome.clientId);
  }

  // The latch after an outcome: any verdict for the in-flight prompt frees it.
  inflightAfterOutcome(inflightId: string | null, outcome: PromptOutcome): string | null {
    return inflightId === outcome.clientId ? null : inflightId;
  }
}

export const promptQueueState = new PromptQueueState();
