import type { ChatMode, ChatProvider, ReasoningEffort } from "../../../models/chat";
import { composerModelOptionsForProvider, MODE_OPTIONS, PROVIDER_OPTIONS, REASONING_EFFORT_OPTIONS } from "../../../state/chat/usage";
import { Clock } from "../../ui/icons";
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
  const modelOptions = composerModelOptionsForProvider(provider);
  return (
    <div class="codex-composer-controls px-3 pt-2 pb-1.5">
      <div class="w-full flex items-center gap-2 flex-wrap">
        <SkillPicker
          provider={provider}
          projectId={projectId}
          onSelect={(skill) => onInsertSkill(skill.name)}
        />

        <label class="codex-model-control inline-flex items-center gap-2 h-9 px-2.5 rounded-md bg-white/[0.05] border border-white/10 text-[12px] text-ink-300 flex-none">
          <span class="hidden sm:inline text-ink-400">Provider</span>
          <select
            value={provider}
            onChange={(event) => onProviderChange((event.currentTarget as HTMLSelectElement).value as ChatProvider)}
            class="bg-transparent text-ink-100 text-[13px] font-medium focus:outline-none"
            title="Provider"
            disabled={streaming}
          >
            {PROVIDER_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>

        <label class="codex-model-control hidden sm:inline-flex items-center gap-2 h-9 px-2.5 rounded-md bg-white/[0.05] border border-white/10 text-[12px] text-ink-300 flex-none">
          <span class="hidden sm:inline text-ink-400">Model</span>
          <select
            value={model}
            onChange={(event) => onModelChange((event.currentTarget as HTMLSelectElement).value)}
            class="bg-transparent text-ink-100 text-[13px] font-medium focus:outline-none"
            title="Model"
            disabled={streaming}
          >
            {model && !modelOptions.some((option) => option.value === model) && (
              <option value={model}>{model}</option>
            )}
            {modelOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>

        {provider === "codex" && (
          <label class="codex-mode-control inline-flex items-center gap-2 h-9 px-2.5 rounded-md bg-white/[0.05] border border-white/10 text-[12px] text-ink-300 flex-none">
            <span class="hidden sm:inline text-ink-400">Thinking</span>
            <select
              value={reasoningEffort}
              onChange={(event) => onReasoningEffortChange((event.currentTarget as HTMLSelectElement).value as ReasoningEffort)}
              class="bg-transparent text-ink-100 text-[13px] font-medium focus:outline-none"
              title="Codex thinking"
              disabled={streaming}
            >
              {REASONING_EFFORT_OPTIONS.map((option) => (
                <option key={option.value || "auto"} value={option.value}>{option.label}</option>
              ))}
            </select>
          </label>
        )}

        <label class="codex-mode-control inline-flex items-center gap-2 h-9 px-2.5 rounded-md bg-white/[0.05] border border-white/10 text-[12px] text-ink-300 flex-none">
          <span class="hidden sm:inline text-ink-400">Mode</span>
          <select
            value={mode}
            onChange={(event) => onModeChange((event.currentTarget as HTMLSelectElement).value as ChatMode)}
            class="bg-transparent text-ink-100 text-[13px] font-medium focus:outline-none"
            title="Mode"
          >
            {MODE_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>

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
