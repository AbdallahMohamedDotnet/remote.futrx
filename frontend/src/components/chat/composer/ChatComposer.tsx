import type { RefObject } from "preact";
import type { ChatMode, ChatProvider, QueuedPrompt, ReasoningEffort } from "../../../models/chat";
import type { Attachment } from "../../../models/upload";
import { AttachmentTray } from "./AttachmentTray";
import { AttachButton } from "./AttachButton";
import { ComposerDropOverlay } from "./ComposerDropOverlay";
import { ComposerToolbar } from "./ComposerToolbar";
import { PromptTextarea } from "./PromptTextarea";
import { QueuedPromptList } from "./QueuedPromptList";
import { SendControls } from "./SendControls";

export function ChatComposer({
  projectId,
  streaming,
  canSendPrompt,
  model,
  provider,
  mode,
  reasoningEffort,
  queuedPrompts,
  attachments,
  uploading,
  dragging,
  text,
  textareaRef,
  fileInputRef,
  onTextChange,
  onFilesSelected,
  onPaste,
  onSend,
  onCancel,
  onRemoveQueued,
  onRemoveAttachment,
  onProviderChange,
  onModelChange,
  onModeChange,
  onReasoningEffortChange,
}: {
  chatId: string;
  projectId?: string;
  streaming: boolean;
  canSendPrompt: boolean;
  model: string;
  provider: ChatProvider;
  mode: ChatMode;
  reasoningEffort: ReasoningEffort;
  queuedPrompts: QueuedPrompt[];
  draftText?: string;
  draftKey?: number;
  attachments: Attachment[];
  uploading: boolean;
  dragging: boolean;
  text: string;
  textareaRef: RefObject<HTMLTextAreaElement>;
  fileInputRef: RefObject<HTMLInputElement>;
  onTextChange: (text: string) => void;
  onFilesSelected: (files: File[]) => void;
  onPaste: (event: ClipboardEvent) => void;
  onSend: () => void;
  onCancel: () => void;
  onRemoveQueued: (id: string) => void;
  onRemoveAttachment: (id: string) => void;
  onProviderChange: (provider: ChatProvider) => void;
  onModelChange: (model: string) => void;
  onModeChange: (mode: ChatMode) => void;
  onReasoningEffortChange: (reasoningEffort: ReasoningEffort) => void;
}) {
  const disconnected = !canSendPrompt && !streaming;
  const hasContent = text.trim().length > 0 || attachments.some((attachment) => attachment.serverPath);
  const canSend = !uploading && !disconnected && hasContent;

  function insertSkill(skillName: string) {
    const mention = `$${skillName} `;
    const textarea = textareaRef.current;
    const start = textarea?.selectionStart ?? text.length;
    const end = textarea?.selectionEnd ?? start;
    const next = `${text.slice(0, start)}${mention}${text.slice(end)}`;
    onTextChange(next);
    window.setTimeout(() => {
      textareaRef.current?.focus();
      textareaRef.current?.setSelectionRange(start + mention.length, start + mention.length);
    }, 0);
  }

  return (
    <div class="codex-composer-shell flex-none z-20 relative bg-[#0b0d11] border-t border-white/10">
      {dragging && <ComposerDropOverlay />}

      <ComposerToolbar
        projectId={projectId}
        model={model}
        provider={provider}
        mode={mode}
        reasoningEffort={reasoningEffort}
        streaming={streaming}
        onInsertSkill={insertSkill}
        onProviderChange={onProviderChange}
        onModelChange={onModelChange}
        onModeChange={onModeChange}
        onReasoningEffortChange={onReasoningEffortChange}
      />

      <QueuedPromptList queuedPrompts={queuedPrompts} onRemove={onRemoveQueued} />
      <AttachmentTray attachments={attachments} onRemove={onRemoveAttachment} />

      <form
        onSubmit={(event) => {
          event.preventDefault();
          onSend();
        }}
        class="codex-composer-form composer-form flex gap-2 items-end px-3 pt-2"
      >
        <AttachButton
          fileInputRef={fileInputRef}
          uploading={uploading}
          disconnected={disconnected}
          onFilesSelected={onFilesSelected}
        />
        <PromptTextarea
          textareaRef={textareaRef}
          text={text}
          uploading={uploading}
          streaming={streaming}
          disconnected={disconnected}
          onTextChange={onTextChange}
          onPaste={onPaste}
          onSend={onSend}
        />
        <SendControls
          streaming={streaming}
          canSend={canSend}
          disconnected={disconnected}
          onCancel={onCancel}
        />
      </form>
    </div>
  );
}
