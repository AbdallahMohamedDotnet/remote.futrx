import type { ChatMode, ChatProvider, ReasoningEffort } from "../../../models/chat";
import { MODE_OPTIONS, modelOptionsForProvider, REASONING_EFFORT_OPTIONS } from "../../../state/chat/usage";
import { Clock } from "../../ui/icons";
import { ComposerModelPicker } from "./ComposerModelPicker";
import { ProviderToggle } from "./ProviderToggle";
import { SegmentedOptionGroup } from "./SegmentedOptionGroup";
import { SkillPicker } from "./SkillPicker";

export function ComposerToolbar({
  projectId,
  model,
  provider,
  mode,
  reasoningEffort,
  streaming,
  onInsertSkill,
  onProviderChange,
  onModelChange,
  onModeChange,
  onReasoningEffortChange,
}: {
  projectId?: string;
  model: string;
  provider: ChatProvider;
  mode: ChatMode;
  reasoningEffort: ReasoningEffort;
  streaming: boolean;
  onInsertSkill: (skillName: string) => void;
  onProviderChange: (provider: ChatProvider) => void;
  onModelChange: (model: string) => void;
  onModeChange: (mode: ChatMode) => void;
  onReasoningEffortChange: (reasoningEffort: ReasoningEffort) => void;
}) {
  const modelOptions = modelOptionsForProvider(provider);
  return (
    <div class="codex-composer-controls px-3 pt-2 pb-1.5">
      <div class="flex w-full flex-col gap-2 lg:flex-row lg:items-start">
        <div class="min-w-0 flex-1 rounded-md border border-white/10 bg-white/[0.035] p-2">
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <ProviderToggle
              provider={provider}
              streaming={streaming}
              onChange={onProviderChange}
            />

            <ComposerModelPicker
              provider={provider}
              model={model}
              streaming={streaming}
              options={modelOptions}
              onChange={onModelChange}
            />
          </div>
        </div>

        <SkillPicker
          provider={provider}
          projectId={projectId}
          onSelect={(skill) => onInsertSkill(skill.name)}
        />
      </div>

      <div class="mt-2 flex w-full min-w-0 flex-wrap items-center gap-2">
        {provider === "codex" && (
          <SegmentedOptionGroup
            label="Thinking"
            value={reasoningEffort}
            options={REASONING_EFFORT_OPTIONS}
            disabled={streaming}
            onChange={(value) => onReasoningEffortChange(value as ReasoningEffort)}
          />
        )}

        <SegmentedOptionGroup
          label="Mode"
          value={mode}
          options={MODE_OPTIONS}
          onChange={(value) => onModeChange(value as ChatMode)}
        />

        {streaming && (
          <div class="inline-flex items-center gap-1.5 h-9 px-2.5 rounded-md bg-accent-blue/[0.12] border border-accent-blue/25 text-[12px] text-accent-blue flex-none">
            <Clock class="w-3.5 h-3.5" />
            <span class="hidden sm:inline">Next send queues</span>
            <span class="sm:hidden">Queues</span>
          </div>
        )}
      </div>
    </div>
  );
}
