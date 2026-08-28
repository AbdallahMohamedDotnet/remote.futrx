import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import type {
  AnswerQuestionHandler,
  ChatStatus,
  InteractionAnswers,
  PromptExecutionPreferences,
  PromptOutcome,
} from "../../../models/chat";
import { useConfirm } from "../../context/ConfirmContext";
import { chatComposerSessionStore } from "../../chat/composerSessionStore";
import { dispatchQuestionAnswer } from "../../chat/chatInteractionState";
import { useAttachmentUpload } from "./useAttachmentUpload";
import { useAutosizeTextarea } from "./useAutosizeTextarea";
import { useDragUpload } from "./useDragUpload";
import { usePromptQueue } from "./usePromptQueue";
import { useThreadScroll } from "./useThreadScroll";

export function useChatComposerController({
  chatId,
  eventCount,
  blockCount,
  status,
  canSendPrompt,
  transportReady,
  sendPrompt,
  executionPreferences,
  sendInteractionResponse,
  promptOutcome,
  rewind,
  refreshMeta,
  attachmentBasePath,
}: {
  chatId: string;
  eventCount: number;
  blockCount: number;
  status: ChatStatus;
  canSendPrompt: boolean;
  transportReady: boolean;
  sendPrompt: (
    text: string,
    preferences: PromptExecutionPreferences,
    clientId?: string,
  ) => boolean;
  executionPreferences: PromptExecutionPreferences;
  sendInteractionResponse: (id: string, answers: InteractionAnswers) => boolean;
  promptOutcome: PromptOutcome | null;
  rewind: (beforeT: number) => Promise<unknown>;
  refreshMeta: () => Promise<void>;
  attachmentBasePath: string;
}) {
  const confirm = useConfirm();
  // Initialise from the per-chat session store and mirror every change back to it.
  // ChatContainer remounts on chat switch (it is keyed by chatId), so this is
  // what makes a half-typed message survive leaving and returning to a chat.
  const [text, setTextState] = useState(() => chatComposerSessionStore.getDraft(chatId));
  const setText = useCallback(
    (value: string | ((prev: string) => string)) => {
      setTextState((prev) => {
        const next = typeof value === "function" ? (value as (prev: string) => string)(prev) : value;
        chatComposerSessionStore.setDraft(chatId, next);
        return next;
      });
    },
    [chatId],
  );
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { textareaRef, focusInput } = useAutosizeTextarea(text);
  const upload = useAttachmentUpload(chatId, attachmentBasePath);
  const drag = useDragUpload(upload.doUpload);
  const scroll = useThreadScroll(chatId, `${eventCount}:${blockCount}`);
  const restorePrompt = useCallback((rejected: string) => {
    setText((current) => current.trim() ? `${rejected}\n\n${current}` : rejected);
    setTimeout(focusInput, 0);
  }, [focusInput, setText]);
  const queue = usePromptQueue({
    chatId,
    status,
    canSendPrompt,
    transportReady,
    sendPrompt,
    promptOutcome,
    onSent: scroll.unlockAutoScroll,
    onRejected: restorePrompt,
  });

  useEffect(() => {
    scroll.unlockAutoScroll();
  }, [chatId]);

  async function handleRewind(t: number, promptText: string) {
    if (status === "streaming") {
      alert("Cancel the current run before rewinding this chat.");
      return;
    }
    const confirmed = await confirm({
      title: "Rewind chat",
      description: "This action cannot be undone.",
      message: "Every message from this prompt forward is removed, and the prompt is put back in the composer.",
      confirmLabel: "Rewind",
    });
    if (!confirmed) return;
    try {
      await rewind(t);
      queue.clearQueuedPrompts();
      setText(promptText);
      await refreshMeta();
      scroll.unlockAutoScroll();
      setTimeout(() => {
        scroll.jumpToBottom();
        focusInput();
      }, 0);
    } catch (rewindError) {
      alert("rewind failed: " + (rewindError as Error).message);
    }
  }

  function handlePaste(event: ClipboardEvent) {
    const items = event.clipboardData?.items;
    if (!items) return;
    const files: File[] = [];
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      if (item.kind === "file") {
        const file = item.getAsFile();
        if (file) files.push(file);
      }
    }
    if (files.length) {
      event.preventDefault();
      upload.doUpload(files);
    }
  }

  function handleSend() {
    if (upload.uploading || (!chatComposerSessionStore.allowsQueue(status) && !canSendPrompt)) return;
    const userText = text.trim();
    const paths = upload.attachments
      .filter((attachment) => attachment.serverPath)
      .map((attachment) => attachment.serverPath);
    if (!userText && paths.length === 0) return;
    const finalText = paths.length
      ? chatComposerSessionStore.promptWithAttachments(userText, paths)
      : userText;

    if (!sendTrackedPrompt(finalText)) return;

    setText("");
    upload.clearAttachments();
    scroll.unlockAutoScroll();
    setTimeout(focusInput, 0);
  }

  function sendTrackedPrompt(promptText: string): boolean {
    if (!chatComposerSessionStore.allowsQueue(status) && !canSendPrompt) return false;
    queue.queuePrompt(promptText, executionPreferences);
    return true;
  }

  const handleAnswerQuestion: AnswerQuestionHandler = (answer) => {
    const sent = dispatchQuestionAnswer(answer, {
      sendPrompt: sendTrackedPrompt,
      sendInteractionResponse,
    });
    if (sent) scroll.unlockAutoScroll();
    return sent;
  };

  return {
    text,
    setText,
    textareaRef,
    fileInputRef,
    upload,
    drag,
    scroll,
    queue,
    handlePaste,
    handleSend,
    handleAnswerQuestion,
    handleRewind,
  };
}
