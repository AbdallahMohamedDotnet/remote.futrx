import type { ChatMode, ChatProvider, ReasoningEffort } from "../../../models/chat";
import { MODE_OPTIONS, REASONING_EFFORT_OPTIONS } from "../../../state/chat/usage";
import { Activity, Clock, MessageSquare } from "../../ui/icons";
import { ComposerOptionDropdown } from "./ComposerOptionDropdown";

export function ComposerOptionsRow({
  provider,
  mode,
  reasoningEffort,
  streaming,
  onModeChange,
  onReasoningEffortChange,
}: {
  provider: ChatProvider;
  mode: ChatMode;
  reasoningEffort: ReasoningEffort;
  streaming: boolean;
  onModeChange: (mode: ChatMode) => void;
  onReasoningEffortChange: (reasoningEffort: ReasoningEffort) => void;
}) {
  return (
    <div class="codex-composer-secondary-controls px-3 pt-1.5 pb-2">
      <div class="flex w-full min-w-0 flex-wrap items-center gap-2">
        {provider === "codex" && (
          <ComposerOptionDropdown
            label="Thinking"
            value={reasoningEffort}
            options={REASONING_EFFORT_OPTIONS}
            disabled={streaming}
            Icon={Activity}
            onChange={(value) => onReasoningEffortChange(value as ReasoningEffort)}
          />
        )}

        <ComposerOptionDropdown
          label="Mode"
          value={mode}
          options={MODE_OPTIONS}
          Icon={MessageSquare}
          onChange={(value) => onModeChange(value as ChatMode)}
        />

        {streaming && (
          <div class="inline-flex h-8 flex-none items-center gap-1.5 rounded-md bg-accent-blue/[0.1] px-2.5 text-[12px] text-accent-blue">
            <Clock class="h-3.5 w-3.5" />
            <span class="hidden sm:inline">Next send queues</span>
            <span class="sm:hidden">Queues</span>
          </div>
        )}
      </div>
    </div>
  );
}
