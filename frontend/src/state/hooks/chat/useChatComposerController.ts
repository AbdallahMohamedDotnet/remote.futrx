import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import type { ChatStatus } from "../../../models/chat";
import { getDraft, setDraft } from "../../chat/drafts";
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
  sendPrompt,
  rewind,
  refreshMeta,
  attachmentBasePath,
}: {
  chatId: string;
  eventCount: number;
  blockCount: number;
  status: ChatStatus;
  canSendPrompt: boolean;
  sendPrompt: (text: string) => boolean;
  rewind: (beforeT: number) => Promise<unknown>;
  refreshMeta: () => Promise<void>;
  attachmentBasePath: string;
}) {
  // Initialise from the per-chat draft store and mirror every change back to it.
  // ChatContainer remounts on chat switch (it is keyed by chatId), so this is
  // what makes a half-typed message survive leaving and returning to a chat.
  const [text, setTextState] = useState(() => getDraft(chatId));
  const setText = useCallback(
    (value: string | ((prev: string) => string)) => {
      setTextState((prev) => {
        const next = typeof value === "function" ? (value as (prev: string) => string)(prev) : value;
        setDraft(chatId, next);
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
  const queue = usePromptQueue({
    chatId,
    status,
    canSendPrompt,
    sendPrompt,
    onSent: scroll.unlockAutoScroll,
  });

  useEffect(() => {
    scroll.unlockAutoScroll();
  }, [chatId]);

  async function handleRewind(t: number, promptText: string) {
    if (status === "streaming") {
      alert("Cancel the current run before rewinding this chat.");
      return;
    }
    if (!confirm("Rewind to this prompt? Messages from this point forward will be removed.")) return;
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
    if (upload.uploading || (!statusAllowsQueue(status) && !canSendPrompt)) return;
    const userText = text.trim();
    const paths = upload.attachments
      .filter((attachment) => attachment.serverPath)
      .map((attachment) => attachment.serverPath);
    if (!userText && paths.length === 0) return;
    const finalText = paths.length
      ? appendAttachmentPaths(userText, paths)
      : userText;

    if (status === "streaming") {
      queue.queuePrompt(finalText);
    } else {
      const sent = sendPrompt(finalText);
      if (!sent) return;
    }

    setText("");
    upload.clearAttachments();
    scroll.unlockAutoScroll();
    setTimeout(focusInput, 0);
  }

  function handleAnswerQuestion(answer: string) {
    const sent = sendPrompt(answer);
    if (sent) scroll.unlockAutoScroll();
  }

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

function statusAllowsQueue(status: ChatStatus): boolean {
  return status === "streaming";
}

function appendAttachmentPaths(userText: string, paths: string[]) {
  const attachmentText = `Attached files:\n${paths.map((path) => `- ${path}`).join("\n")}`;
  return userText ? `${userText}\n\n${attachmentText}` : attachmentText;
}
