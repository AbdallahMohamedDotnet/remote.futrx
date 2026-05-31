import type { RefObject } from "preact";
import type { ChatMeta, ChatMode, ChatProvider, ChatStatus, QueuedPrompt } from "../../models/chat";
import type { Attachment } from "../../models/upload";
import type { Block } from "../../state/chat/messageBlocks";
import type { UsageTotals } from "../../state/chat/usage";
import { ChatComposer } from "./ChatComposer";
import { JumpToLatestButton } from "./JumpToLatestButton";
import { MessageList } from "./MessageList";
import { ThreadHeader } from "./ThreadHeader";

export function ChatThread({
  chat,
  blocks,
  hasOlder,
  loadingOlder,
  status,
  error,
  canSendPrompt,
  streaming,
  mode,
  queuedPrompts,
  draftText,
  draftKey,
  attachments,
  uploading,
  dragging,
  text,
  textareaRef,
  fileInputRef,
  showJump,
  scrollRef,
  contentRef,
  bottomRef,
  header,
  usageTotals,
  tokenLabel,
  costUsd,
  onHamburger,
  onScroll,
  onJumpToBottom,
  onAnswerQuestion,
  onLoadOlder,
  onRewind,
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
  onOpenTerminal,
  onOpenDatabase,
  openingDatabase,
}: {
  chat: ChatMeta;
  blocks: Block[];
  hasOlder: boolean;
  loadingOlder: boolean;
  status: ChatStatus;
  error: string | null;
  canSendPrompt: boolean;
  streaming: boolean;
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
  showJump: boolean;
  scrollRef: RefObject<HTMLDivElement>;
  contentRef: RefObject<HTMLDivElement>;
  bottomRef: RefObject<HTMLDivElement>;
  header: {
    modelRef: RefObject<HTMLDivElement>;
    modelOpen: boolean;
    modelOptions: Array<{ value: string; label: string; sub: string }>;
    modelDisplayLabel: (model?: string) => string;
    editingCwd: boolean;
    cwdInput: string;
    onToggleModel: () => void;
    onPickModel: (model: string) => void;
    onStartEditCwd: () => void;
    onCwdInput: (value: string) => void;
    onCommitCwd: () => void;
    onCancelCwdEdit: () => void;
  };
  usageTotals: UsageTotals;
  tokenLabel: string;
  costUsd: number;
  onHamburger: () => void;
  onScroll: () => void;
  onJumpToBottom: () => void;
  onAnswerQuestion: (text: string) => void;
  onLoadOlder: () => Promise<void>;
  onRewind: (t: number, text: string) => void;
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
  onOpenTerminal: () => void;
  onOpenDatabase: () => void;
  openingDatabase: boolean;
}) {
  return (
    <div class="codex-thread flex-1 h-full flex flex-col min-h-0 overflow-hidden bg-[#0b0d11]">
      <ThreadHeader
        chat={chat}
        streaming={streaming}
        modelRef={header.modelRef}
        modelOpen={header.modelOpen}
        modelOptions={header.modelOptions}
        modelDisplayLabel={header.modelDisplayLabel}
        editingCwd={header.editingCwd}
        cwdInput={header.cwdInput}
        usageTotals={usageTotals}
        tokenLabel={tokenLabel}
        costUsd={costUsd}
        onToggleModel={header.onToggleModel}
        onPickModel={header.onPickModel}
        onStartEditCwd={header.onStartEditCwd}
        onCwdInput={header.onCwdInput}
        onCommitCwd={header.onCommitCwd}
        onCancelCwdEdit={header.onCancelCwdEdit}
        onOpenTerminal={onOpenTerminal}
        onOpenDatabase={onOpenDatabase}
        openingDatabase={openingDatabase}
        onHamburger={onHamburger}
      />

      <div class="relative flex-1 min-h-0">
        <MessageList
          status={status}
          blocks={blocks}
          hasOlder={hasOlder}
          loadingOlder={loadingOlder}
          error={error}
          chatId={chat.id}
          cwd={chat.cwd}
          scrollRef={scrollRef}
          contentRef={contentRef}
          bottomRef={bottomRef}
          onScroll={onScroll}
          onAnswerQuestion={onAnswerQuestion}
          onLoadOlder={onLoadOlder}
          onRewind={onRewind}
        />
        {showJump && <JumpToLatestButton onClick={onJumpToBottom} />}
      </div>

      <ChatComposer
        chatId={chat.id}
        streaming={streaming}
        canSendPrompt={canSendPrompt}
        model={chat.model || ""}
        provider={chat.provider || "claude"}
        mode={mode}
        queuedPrompts={queuedPrompts}
        draftText={draftText}
        draftKey={draftKey}
        attachments={attachments}
        uploading={uploading}
        dragging={dragging}
        text={text}
        textareaRef={textareaRef}
        fileInputRef={fileInputRef}
        onTextChange={onTextChange}
        onFilesSelected={onFilesSelected}
        onPaste={onPaste}
        onSend={onSend}
        onCancel={onCancel}
        onRemoveQueued={onRemoveQueued}
        onRemoveAttachment={onRemoveAttachment}
        onProviderChange={onProviderChange}
        onModelChange={onModelChange}
        onModeChange={onModeChange}
      />
    </div>
  );
}
