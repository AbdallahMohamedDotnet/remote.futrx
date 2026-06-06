import type { RefObject } from "preact";
import type { ChatMeta, ChatMode, ChatProvider, ChatStatus, QueuedPrompt, ReasoningEffort, SelectedSkill } from "../../models/chat";
import type { Attachment } from "../../models/upload";
import type { RegisteredSkill } from "../../models/skill";
import type { Block } from "../../state/chat/messageBlocks";
import { ChatComposer } from "./composer/ChatComposer";
import { JumpToLatestButton } from "./messages/JumpToLatestButton";
import { MessageList } from "./messages/MessageList";
import { ThreadHeader } from "./header/ThreadHeader";

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
  selectedSkills,
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
  onReasoningEffortChange,
  onSelectSkill,
  onRemoveSelectedSkill,
  onOpenTerminal,
  onOpenBrowser,
  onOpenHistory,
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
  selectedSkills: SelectedSkill[];
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
    editingCwd: boolean;
    cwdInput: string;
    onStartEditCwd: () => void;
    onCwdInput: (value: string) => void;
    onCommitCwd: () => void;
    onCancelCwdEdit: () => void;
  };
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
  onReasoningEffortChange: (reasoningEffort: ReasoningEffort) => void;
  onSelectSkill: (skill: RegisteredSkill) => void;
  onRemoveSelectedSkill: (skill: SelectedSkill) => void;
  onOpenTerminal: () => void;
  onOpenBrowser: () => void;
  onOpenHistory: () => void;
}) {
  return (
    <div class="codex-thread flex-1 h-full flex flex-col min-h-0 overflow-hidden bg-[#0b0d11]">
      <ThreadHeader
        chat={chat}
        streaming={streaming}
        editingCwd={header.editingCwd}
        cwdInput={header.cwdInput}
        onStartEditCwd={header.onStartEditCwd}
        onCwdInput={header.onCwdInput}
        onCommitCwd={header.onCommitCwd}
        onCancelCwdEdit={header.onCancelCwdEdit}
        onOpenTerminal={onOpenTerminal}
        onOpenBrowser={onOpenBrowser}
        onOpenHistory={onOpenHistory}
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
        projectId={chat.projectId}
        streaming={streaming}
        canSendPrompt={canSendPrompt}
        model={chat.model || ""}
        provider={chat.provider || "codex"}
        mode={mode}
        reasoningEffort={chat.reasoningEffort || ""}
        queuedPrompts={queuedPrompts}
        selectedSkills={selectedSkills}
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
        onReasoningEffortChange={onReasoningEffortChange}
        onSelectSkill={onSelectSkill}
        onRemoveSelectedSkill={onRemoveSelectedSkill}
      />
    </div>
  );
}
