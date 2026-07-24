import { useEffect, useState } from "preact/hooks";
import type { ChatStatus, PromptOutcome, QueuedPrompt } from "../../../models/chat";
import { queueId } from "../../../shared/ids";
import { chatComposerSessionStore } from "../../chat/composerSessionStore";
import { promptQueueState } from "../../chat/promptQueueState";

export function usePromptQueue({
  chatId,
  status,
  canSendPrompt,
  sendPrompt,
  promptOutcome,
  onSent,
}: {
  chatId: string;
  status: ChatStatus;
  canSendPrompt: boolean;
  sendPrompt: (text: string, clientId?: string) => boolean;
  promptOutcome: PromptOutcome | null;
  onSent: () => void;
}) {
  // Retained per chat in the session store so queued prompts survive the
  // ChatContainer remount that happens on every chat switch. They resume
  // auto-sending when you return to the chat (sending is tied to the active
  // chat's connection, so a backgrounded chat's queue waits until it is open).
  const [queuedPrompts, setQueuedPromptsState] = useState<QueuedPrompt[]>(() =>
    chatComposerSessionStore.getQueuedPrompts(chatId),
  );
  // Dispatch latch: the queued prompt currently on the wire awaiting the
  // server's verdict. Deliberately not persisted — the prompt itself stays
  // queued until accepted, so losing the latch can at worst re-send it.
  const [inflightId, setInflightId] = useState<string | null>(null);

  function commitQueuedPrompts(updater: QueuedPrompt[] | ((prev: QueuedPrompt[]) => QueuedPrompt[])) {
    setQueuedPromptsState((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      chatComposerSessionStore.setQueuedPrompts(chatId, next);
      return next;
    });
  }

  // A dispatched prompt is removed only when the server accepts it; a
  // rejection (run lock still held) keeps it queued for the next window.
  useEffect(() => {
    if (!promptOutcome) return;
    setInflightId((current) => promptQueueState.inflightAfterOutcome(current, promptOutcome));
    if (promptOutcome.accepted) {
      commitQueuedPrompts((prev) => promptQueueState.promptsAfterOutcome(prev, promptOutcome));
      onSent();
    }
  }, [promptOutcome]);

  // The send window closing resolves any dispatch: its verdict either already
  // arrived or will never arrive on this connection, so free the latch.
  useEffect(() => {
    if (status !== "ready" || !canSendPrompt) setInflightId(null);
  }, [status, canSendPrompt]);

  useEffect(() => {
    const next = promptQueueState.nextDispatch(queuedPrompts, inflightId, status, canSendPrompt);
    if (!next) return;
    if (!sendPrompt(next.text, next.id)) return;
    setInflightId(next.id);
  }, [status, canSendPrompt, queuedPrompts, inflightId, sendPrompt]);

  return {
    queuedPrompts,
    queuePrompt: (text: string) =>
      commitQueuedPrompts((prev) => [...prev, { id: queueId(), text }]),
    removeQueuedPrompt: (id: string) =>
      commitQueuedPrompts((prev) => prev.filter((prompt) => prompt.id !== id)),
    clearQueuedPrompts: () => commitQueuedPrompts([]),
  };
}
