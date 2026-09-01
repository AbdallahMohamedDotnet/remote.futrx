import { useStore } from "zustand";
import { useEffect, useState } from "preact/hooks";
import type {
  ChatStatus,
  PromptExecutionPreferences,
  PromptOutcome,
  QueuedPrompt,
} from "../../../models/chat";
import { idService } from "../../../services/platform/idService.ts";
import { chatComposerSessionStore } from "../../stores/chat/composerSessionStore";
import { promptQueueState } from "./promptQueueState";

const EMPTY_QUEUE: QueuedPrompt[] = [];

export function usePromptQueue({
  chatId,
  status,
  canSendPrompt,
  transportReady,
  sendPrompt,
  promptOutcome,
  onSent,
  onRejected,
}: {
  chatId: string;
  status: ChatStatus;
  canSendPrompt: boolean;
  transportReady: boolean;
  sendPrompt: (
    text: string,
    preferences: PromptExecutionPreferences,
    clientId?: string,
  ) => boolean;
  promptOutcome: PromptOutcome | null;
  onSent: () => void;
  onRejected: (text: string) => void;
}) {
  // Retained per chat in the session store so queued prompts survive the
  // ChatContainer remount that happens on every chat switch. They resume
  // auto-sending when you return to the chat (sending is tied to the active
  // chat's connection, so a backgrounded chat's queue waits until it is open).
  const queuedPrompts = useStore(
    chatComposerSessionStore,
    (state) => state.promptQueues.get(chatId) ?? EMPTY_QUEUE,
  );
  const setQueuedPrompts = useStore(
    chatComposerSessionStore,
    (state) => state.setQueuedPrompts,
  );
  // Dispatch latch: the queued prompt currently on the wire awaiting the
  // server's verdict. Deliberately not persisted — the prompt itself stays
  // queued until accepted, so losing the latch can at worst re-send it.
  const [inflightId, setInflightId] = useState<string | null>(null);

  function commitQueuedPrompts(updater: QueuedPrompt[] | ((prev: QueuedPrompt[]) => QueuedPrompt[])) {
    const previous = chatComposerSessionStore.getState().promptQueues.get(chatId) ?? EMPTY_QUEUE;
    const next = typeof updater === "function" ? updater(previous) : updater;
    setQueuedPrompts(chatId, next);
  }

  // A busy rejection remains queued. Semantic rejections are removed and
  // returned to the draft for an explicit resend after the user reviews the
  // current provider/mode.
  useEffect(() => {
    if (!promptOutcome) return;
    // Resolve the delivery latch even if another UI action already removed
    // the queue chip. Otherwise a late ack for that deleted item can block the
    // rest of the queue until reconnect.
    setInflightId((current) => promptQueueState.inflightAfterOutcome(current, promptOutcome));
    const matched = queuedPrompts.find((prompt) => prompt.id === promptOutcome.clientId);
    if (!matched) return;
    if (promptOutcome.accepted) {
      commitQueuedPrompts((prev) => promptQueueState.promptsAfterOutcome(prev, promptOutcome));
      onSent();
    } else if (!promptOutcome.retryable) {
      commitQueuedPrompts((prev) => promptQueueState.promptsAfterOutcome(prev, promptOutcome));
      onRejected(matched.text);
    }
  }, [promptOutcome, queuedPrompts]);

  // A normal run closes the send window before its ack arrives, so keep the
  // latch through streaming/ready transitions. Only losing the transport makes
  // the verdict indeterminate; durable server-side client IDs make retry after
  // reconnect safe.
  useEffect(() => {
    if (!transportReady) setInflightId(null);
  }, [transportReady]);

  useEffect(() => {
    const next = promptQueueState.nextDispatch(queuedPrompts, inflightId, status, canSendPrompt);
    if (!next) return;
    if (!sendPrompt(next.text, next.preferences, next.id)) return;
    setInflightId(next.id);
  }, [status, canSendPrompt, queuedPrompts, inflightId, sendPrompt]);

  return {
    queuedPrompts,
    inflightId,
    queuePrompt: (text: string, preferences: PromptExecutionPreferences) =>
      commitQueuedPrompts((prev) => [...prev, {
        id: idService.timeOrdered(),
        text,
        preferences: { ...preferences },
      }]),
    removeQueuedPrompt: (id: string) => {
      if (id === inflightId) return;
      commitQueuedPrompts((prev) => prev.filter((prompt) => prompt.id !== id));
    },
    clearQueuedPrompts: () => commitQueuedPrompts([]),
  };
}
