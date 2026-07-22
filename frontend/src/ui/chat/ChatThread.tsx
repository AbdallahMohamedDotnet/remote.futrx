import type { RefObject } from "preact";
import type { ChatMeta, ChatMode, ChatStatus, QueuedPrompt, SelectedSkill } from "../../models/chat";
import type { ChatMessageBlock } from "../../models/chatMessage";
import type { Attachment } from "../../models/upload";
import type { RegisteredSkill } from "../../models/skill";
import { ChatComposer } from "./composer/ChatComposer";
import type { ComposerPreferenceActions, ComposerPreferences } from "./composer/preferences";
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
  composerPreferenceActions,
  onSelectSkill,
  onRemoveSelectedSkill,
  onOpenTerminal,
  onOpenBrowser,
  onOpenHistory,
  onOpenFiles,
}: {
  chat: ChatMeta;
  blocks: ChatMessageBlock[];
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
  composerPreferenceActions: ComposerPreferenceActions;
  onSelectSkill: (skill: RegisteredSkill) => void;
  onRemoveSelectedSkill: (skill: SelectedSkill) => void;
  onOpenTerminal: () => void;
  onOpenBrowser: () => void;
  onOpenHistory: () => void;
  onOpenFiles: () => void;
}) {
  const composerPreferences: ComposerPreferences = {
    provider: chat.provider || "codex",
    model: chat.model || "",
    mode,
    reasoningEffort: chat.reasoningEffort || "",
    serviceTier: chat.serviceTier || "",
  };

  return (
    <div class="codex-thread flex-1 h-full flex flex-col min-h-0 overflow-hidden bg-[#0b0d11]">
      <ThreadHeader
        chat={chat}
        streaming={streaming}
        onOpenTerminal={onOpenTerminal}
        onOpenBrowser={onOpenBrowser}
        onOpenHistory={onOpenHistory}
        onOpenFiles={onOpenFiles}
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
        preferences={composerPreferences}
        preferenceActions={composerPreferenceActions}
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
        onSelectSkill={onSelectSkill}
        onRemoveSelectedSkill={onRemoveSelectedSkill}
      />
    </div>
  );
}
