import { useEffect, useState } from "preact/hooks";
import type { ChatStatus, QueuedPrompt } from "../../models/chat";
import { queueId } from "../../lib/ids";

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
  const [queuedPrompts, setQueuedPrompts] = useState<QueuedPrompt[]>([]);

  useEffect(() => {
    setQueuedPrompts([]);
  }, [chatId]);

  useEffect(() => {
    if (status !== "ready" || !canSendPrompt || queuedPrompts.length === 0) return;
    const next = queuedPrompts[0];
    const sent = sendPrompt(next.text);
    if (!sent) return;
    setQueuedPrompts((prev) => prev.filter((prompt) => prompt.id !== next.id));
    onSent();
  }, [status, canSendPrompt, queuedPrompts, sendPrompt, onSent]);

  return {
    queuedPrompts,
    queuePrompt: (text: string) =>
      setQueuedPrompts((prev) => [...prev, { id: queueId(), text }]),
    removeQueuedPrompt: (id: string) =>
      setQueuedPrompts((prev) => prev.filter((prompt) => prompt.id !== id)),
    clearQueuedPrompts: () => setQueuedPrompts([]),
  };
}
