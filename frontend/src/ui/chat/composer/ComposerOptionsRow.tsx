import type { ChatMode, ChatProvider, ReasoningEffort, ServiceTier } from "../../../models/chat";
import {
  MODE_OPTIONS,
  reasoningEffortOptionsForProvider,
  serviceTierOptionsForProvider,
} from "../../../config/chat";
import { Activity, Cpu, MessageSquare } from "../../primitives/icons";
import { ComposerOptionDropdown } from "./ComposerOptionDropdown";

export function ComposerOptionsRow({
  provider,
  mode,
  reasoningEffort,
  serviceTier,
  streaming,
  onModeChange,
  onReasoningEffortChange,
  onServiceTierChange,
}: {
  provider: ChatProvider;
  mode: ChatMode;
  reasoningEffort: ReasoningEffort;
  serviceTier: ServiceTier;
  streaming: boolean;
  onModeChange: (mode: ChatMode) => void;
  onReasoningEffortChange: (reasoningEffort: ReasoningEffort) => void;
  onServiceTierChange: (serviceTier: ServiceTier) => void;
}) {
  const reasoningEffortOptions = reasoningEffortOptionsForProvider(provider);
  const serviceTierOptions = serviceTierOptionsForProvider(provider);

  return (
    <div class="codex-composer-secondary-controls px-3 pt-1.5 pb-2">
      <div class="flex w-full min-w-0 flex-wrap items-center gap-2">
        {reasoningEffortOptions.length > 0 && (
          <ComposerOptionDropdown
            label="Thinking"
            value={reasoningEffort}
            options={reasoningEffortOptions}
            disabled={streaming}
            Icon={Activity}
            onChange={(value) => onReasoningEffortChange(value as ReasoningEffort)}
          />
        )}

        {serviceTierOptions.length > 0 && (
          <ComposerOptionDropdown
            label="Speed"
            value={serviceTier}
            options={serviceTierOptions}
            disabled={streaming}
            Icon={Cpu}
            onChange={(value) => onServiceTierChange(value as ServiceTier)}
          />
        )}

        <ComposerOptionDropdown
          label="Mode"
          value={mode}
          options={MODE_OPTIONS}
          Icon={MessageSquare}
          onChange={(value) => onModeChange(value as ChatMode)}
        />
      </div>
    </div>
  );
}
