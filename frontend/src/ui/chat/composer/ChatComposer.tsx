import type { RefObject } from "preact";
import { useEffect } from "preact/hooks";
import type { QueuedPrompt, SelectedSkill } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import type { Attachment } from "../../../models/upload";
import { useComposerAgentCapabilities } from "../../../state/hooks/chat/useComposerAgentCapabilities";
import { AttachmentTray } from "./AttachmentTray";
import { AttachButton } from "./AttachButton";
import { ComposerDropOverlay } from "./ComposerDropOverlay";
import { ComposerOptionsRow } from "./ComposerOptionsRow";
import { ComposerToolbar } from "./ComposerToolbar";
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
  const capabilityState = useComposerAgentCapabilities({
    projectId,
    provider: preferences.provider,
    model: preferences.model,
  });
  const {
    providerCapabilities,
    providerOptions,
    modelOptions,
    reasoningEffortOptions,
    serviceTierOptions,
    modeOptions,
  } = capabilityState;

  useEffect(() => {
    if (!providerCapabilities || providerCapabilities.source !== "live") return;
    if (
      preferences.reasoningEffort &&
      !reasoningEffortOptions.some(
        (option) => option.value === preferences.reasoningEffort,
      )
    ) {
      preferenceActions.changeReasoningEffort("");
    }
    if (
      preferences.serviceTier &&
      !serviceTierOptions.some(
        (option) => option.value === preferences.serviceTier,
      )
    ) {
      preferenceActions.changeServiceTier("");
    }
    if (preferences.mode && !modeOptions.some((option) => option.value === preferences.mode)) {
      preferenceActions.changeMode(
        providerCapabilities.defaultMode || modeOptions[0]?.value || "code",
      );
    }
  }, [
    modeOptions,
    preferences.mode,
    preferences.model,
    preferences.reasoningEffort,
    preferences.serviceTier,
    providerCapabilities,
    reasoningEffortOptions,
    serviceTierOptions,
  ]);

  const disconnected = !canSendPrompt && !streaming;
  const hasContent = text.trim().length > 0 || attachments.some((attachment) => attachment.serverPath);
  const canSend = !uploading && !disconnected && hasContent;

  return (
    <div class="codex-composer-shell flex-none z-20 relative bg-[#0b0d11] border-t border-white/10">
      {dragging && <ComposerDropOverlay />}

      <ComposerToolbar
        projectId={projectId}
        model={preferences.model}
        provider={preferences.provider}
        streaming={streaming}
        providerOptions={providerOptions}
        modelOptions={modelOptions}
        selectedSkills={selectedSkills}
        onSelectSkill={onSelectSkill}
        onProviderChange={preferenceActions.changeProvider}
        onModelChange={preferenceActions.changeModel}
      />

      <SelectedSkillChips skills={selectedSkills} onRemove={onRemoveSelectedSkill} />
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

      <ComposerOptionsRow
        preferences={preferences}
        preferenceActions={preferenceActions}
        streaming={streaming}
        reasoningEffortOptions={reasoningEffortOptions}
        serviceTierOptions={serviceTierOptions}
        modeOptions={modeOptions}
      />
    </div>
  );
}
