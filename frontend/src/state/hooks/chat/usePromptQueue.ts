import { useEffect, useState } from "preact/hooks";
import type { ChatStatus, QueuedPrompt } from "../../../models/chat";
import { queueId } from "../../../shared/ids";
import { chatComposerSessionStore } from "../../chat/composerSessionStore";

export function usePromptQueue({
  chatId,
  status,
  canSendPrompt,
  sendPrompt,
  onSent,
}: {
  chatId: string;
  status: ChatStatus;
  canSendPrompt: boolean;
  sendPrompt: (text: string) => boolean;
  onSent: () => void;
}) {
  // Retained per chat in the session store so queued prompts survive the
  // ChatContainer remount that happens on every chat switch. They resume
  // auto-sending when you return to the chat (sending is tied to the active
  // chat's connection, so a backgrounded chat's queue waits until it is open).
  const [queuedPrompts, setQueuedPromptsState] = useState<QueuedPrompt[]>(() =>
    chatComposerSessionStore.getQueuedPrompts(chatId),
  );

  function commitQueuedPrompts(updater: QueuedPrompt[] | ((prev: QueuedPrompt[]) => QueuedPrompt[])) {
    setQueuedPromptsState((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      chatComposerSessionStore.setQueuedPrompts(chatId, next);
      return next;
    });
  }

  useEffect(() => {
    if (status !== "ready" || !canSendPrompt || queuedPrompts.length === 0) return;
    const next = queuedPrompts[0];
    const sent = sendPrompt(next.text);
    if (!sent) return;
    commitQueuedPrompts((prev) => prev.filter((prompt) => prompt.id !== next.id));
    onSent();
  }, [status, canSendPrompt, queuedPrompts, sendPrompt, onSent]);

  return {
    queuedPrompts,
    queuePrompt: (text: string) =>
      commitQueuedPrompts((prev) => [...prev, { id: queueId(), text }]),
    removeQueuedPrompt: (id: string) =>
      commitQueuedPrompts((prev) => prev.filter((prompt) => prompt.id !== id)),
    clearQueuedPrompts: () => commitQueuedPrompts([]),
  };
}
