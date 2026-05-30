import { useEffect, useRef, useState } from "preact/hooks";
import type { ChatMeta, ChatMode } from "../models/chat";
import { ChatThread } from "../components/chat/ChatThread";
import { useChat } from "../hooks/chat/useChat";
import { useAttachmentUpload } from "../hooks/chat/useAttachmentUpload";
import { useAutosizeTextarea } from "../hooks/chat/useAutosizeTextarea";
import { useChatMetaActions } from "../hooks/chat/useChatMetaActions";
import { useDragUpload } from "../hooks/chat/useDragUpload";
import { usePromptQueue } from "../hooks/chat/usePromptQueue";
import { useThreadHeaderState } from "../hooks/chat/useThreadHeaderState";
import { useThreadScroll } from "../hooks/chat/useThreadScroll";
import {
  estimateCost,
  formatTokens,
  MODEL_OPTIONS,
  modelDisplayLabel,
  tokenTotal,
} from "../state/chat/usage";

export function ChatContainer({
  chat,
  onHamburger,
  onMetaUpdate,
}: {
  chat: ChatMeta;
  onHamburger: () => void;
  onMetaUpdate: () => void;
}) {
  const {
    meta,
    blocks,
    usageTotals,
    eventCount,
    status,
    error,
    canSendPrompt,
    sendPrompt,
    cancel,
    rewind,
    refreshMeta,
  } = useChat(chat.id);
  const displayMeta = meta ?? chat;
  const displayMode = displayMeta.mode || "code";
  const [text, setText] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { textareaRef, focusInput } = useAutosizeTextarea(text);
  const upload = useAttachmentUpload(chat.id, onMetaUpdate);
  const drag = useDragUpload(upload.doUpload);
  const scroll = useThreadScroll(chat.id, `${eventCount}:${blocks.length}`);
  const metaActions = useChatMetaActions({
    chatId: chat.id,
    refreshMeta,
    onMetaUpdate,
  });
  const header = useThreadHeaderState(displayMeta.cwd, (cwd) => metaActions.applyMeta({ cwd }));
  const queue = usePromptQueue({
    chatId: chat.id,
    status,
    canSendPrompt,
    sendPrompt,
    onSent: scroll.unlockAutoScroll,
  });
  const costUsd = estimateCost(usageTotals, displayMeta.model || "");
  const tokenLabel = formatTokens(tokenTotal(usageTotals));

  useEffect(() => {
    setText("");
    scroll.unlockAutoScroll();
  }, [chat.id]);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape" && status === "streaming") cancel();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [status, cancel]);

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
      onMetaUpdate();
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
      ? (userText ? `${userText}\n\n${paths.join(" ")}` : paths.join(" "))
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

  function pickModel(model: string) {
    header.setModelOpen(false);
    if (model !== displayMeta.model) metaActions.applyMeta({ model });
  }

  function changeMode(mode: ChatMode) {
    metaActions.applyMeta({ mode });
  }

  return (
    <ChatThread
      chat={displayMeta}
      blocks={blocks}
      status={status}
      error={error}
      canSendPrompt={canSendPrompt}
      streaming={status === "streaming"}
      mode={displayMode}
      queuedPrompts={queue.queuedPrompts}
      attachments={upload.attachments}
      uploading={upload.uploading}
      dragging={drag.dragging}
      text={text}
      textareaRef={textareaRef}
      fileInputRef={fileInputRef}
      showJump={scroll.showJump}
      scrollRef={scroll.scrollRef}
      contentRef={scroll.contentRef}
      bottomRef={scroll.bottomRef}
      header={{
        modelRef: header.modelRef,
        modelOpen: header.modelOpen,
        modelOptions: MODEL_OPTIONS,
        modelDisplayLabel,
        editingCwd: header.editingCwd,
        cwdInput: header.cwdInput,
        onToggleModel: () => header.setModelOpen(!header.modelOpen),
        onPickModel: pickModel,
        onStartEditCwd: () => header.setEditingCwd(true),
        onCwdInput: header.setCwdInput,
        onCommitCwd: header.commitCwd,
        onCancelCwdEdit: header.cancelCwdEdit,
      }}
      usageTotals={usageTotals}
      tokenLabel={tokenLabel}
      costUsd={costUsd}
      onHamburger={onHamburger}
      onScroll={scroll.onScroll}
      onJumpToBottom={scroll.jumpToBottom}
      onAnswerQuestion={(answer) => {
        const sent = sendPrompt(answer);
        if (sent) scroll.unlockAutoScroll();
      }}
      onRewind={handleRewind}
      onTextChange={setText}
      onFilesSelected={upload.doUpload}
      onPaste={handlePaste}
      onSend={handleSend}
      onCancel={cancel}
      onRemoveQueued={queue.removeQueuedPrompt}
      onRemoveAttachment={upload.removeAttachment}
      onModelChange={(model) => metaActions.applyMeta({ model })}
      onModeChange={changeMode}
    />
  );
}

function statusAllowsQueue(status: string): boolean {
  return status === "streaming";
}
