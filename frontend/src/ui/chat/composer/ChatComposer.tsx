import type { RefObject } from "preact";
import type { QueuedPrompt, SelectedSkill } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import type { Attachment } from "../../../models/upload";
import { AttachmentTray } from "./AttachmentTray";
import { AttachButton } from "./AttachButton";
import { ComposerAgentControls } from "./ComposerAgentControls";
import { ComposerDropOverlay } from "./ComposerDropOverlay";
import { ComposerExecutionControls } from "./ComposerExecutionControls";
import { PromptTextarea } from "./PromptTextarea";
import { QueuedPromptList } from "./QueuedPromptList";
import { SelectedSkillChips } from "./SelectedSkillChips";
import { SendControls } from "./SendControls";
import type { ComposerPreferenceActions, ComposerPreferences } from "./preferences";

export interface ChatComposerProps {
  projectId?: string;
  streaming: boolean;
  canSendPrompt: boolean;
  preferences: ComposerPreferences;
  preferenceActions: ComposerPreferenceActions;
  queuedPrompts: QueuedPrompt[];
  selectedSkills: SelectedSkill[];
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
  onSelectSkill: (skill: RegisteredSkill) => void;
  onRemoveSelectedSkill: (skill: SelectedSkill) => void;
}

export function ChatComposer({
  projectId,
  streaming,
  canSendPrompt,
  preferences,
  preferenceActions,
  queuedPrompts,
  selectedSkills,
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
  onSelectSkill,
  onRemoveSelectedSkill,
}: ChatComposerProps) {
  const disconnected = !canSendPrompt && !streaming;
  const hasContent = text.trim().length > 0 || attachments.some((attachment) => attachment.serverPath);
  const canSend = !uploading && !disconnected && hasContent;

  return (
    <div class="codex-composer-shell flex-none z-20 relative bg-[#0b0d11] border-t border-white/10">
      {dragging && <ComposerDropOverlay />}

      <SelectedSkillChips skills={selectedSkills} onRemove={onRemoveSelectedSkill} />
      <QueuedPromptList queuedPrompts={queuedPrompts} onRemove={onRemoveQueued} />
      <AttachmentTray attachments={attachments} onRemove={onRemoveAttachment} />

      <div class="codex-composer-card mx-3 my-2 overflow-visible rounded-xl border border-white/10 bg-[#15171c] shadow-[0_8px_24px_rgba(0,0,0,0.18)]">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            onSend();
          }}
          class="codex-composer-form composer-form flex gap-1.5 items-end px-2 pt-2"
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

        <div class="codex-composer-control-deck flex min-w-0 flex-wrap items-center justify-between gap-1.5 border-t border-white/[0.07] px-2 py-1.5">
          <ComposerAgentControls
            projectId={projectId}
            model={preferences.model}
            provider={preferences.provider}
            streaming={streaming}
            selectedSkills={selectedSkills}
            onSelectSkill={onSelectSkill}
            onProviderChange={preferenceActions.changeProvider}
            onModelChange={preferenceActions.changeModel}
          />

          <ComposerExecutionControls
            preferences={preferences}
            preferenceActions={preferenceActions}
            streaming={streaming}
          />
        </div>
      </div>
    </div>
  );
}
