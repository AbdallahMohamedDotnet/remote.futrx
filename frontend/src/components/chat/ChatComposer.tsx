import type { RefObject } from "preact";
import type { ChatMode, ChatProvider, QueuedPrompt } from "../../models/chat";
import type { Attachment } from "../../models/upload";
import { Plus, Upload } from "../ui/icons";
import { AttachmentTray } from "./AttachmentTray";
import { ComposerToolbar } from "./ComposerToolbar";
import { PromptTextarea } from "./PromptTextarea";
import { QueuedPromptList } from "./QueuedPromptList";
import { SendControls } from "./SendControls";

export function ChatComposer({
  streaming,
  canSendPrompt,
  model,
  provider,
  mode,
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
}: {
  chatId: string;
  streaming: boolean;
  canSendPrompt: boolean;
  model: string;
  provider: ChatProvider;
  mode: ChatMode;
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
}) {
  const disconnected = !canSendPrompt && !streaming;
  const hasContent = text.trim().length > 0 || attachments.some((attachment) => attachment.serverPath);
  const canSend = !uploading && !disconnected && hasContent;

  return (
    <div class="codex-composer-shell flex-none z-20 relative bg-[#0b0d11] border-t border-white/10">
      {dragging && (
        <div class="absolute inset-x-3 -top-16 z-20 rounded-lg border-2 border-dashed border-accent-blue
                    bg-[#151922] text-accent-blue text-sm flex items-center justify-center
                    h-14 gap-2">
          <Upload class="w-5 h-5" />
          Drop files to upload to the chat directory
        </div>
      )}

      <ComposerToolbar
        model={model}
        provider={provider}
        mode={mode}
        streaming={streaming}
        onProviderChange={onProviderChange}
        onModelChange={onModelChange}
        onModeChange={onModeChange}
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
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={uploading || disconnected}
          class="codex-icon-button flex-none w-11 h-11 rounded-md bg-white/[0.06] border border-white/10
                 hover:bg-white/10 active:bg-accent-blue active:border-accent-blue active:scale-[0.98]
                 disabled:opacity-50 disabled:active:scale-100 grid place-items-center text-ink-100 transition"
          aria-label="Attach files"
          title="Attach (or drag-and-drop / paste images)"
        >
          <Plus class="w-4 h-4" />
        </button>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          class="hidden"
          onChange={(event) => {
            const input = event.currentTarget as HTMLInputElement;
            const files = Array.from(input.files || []);
            input.value = "";
            onFilesSelected(files);
          }}
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
