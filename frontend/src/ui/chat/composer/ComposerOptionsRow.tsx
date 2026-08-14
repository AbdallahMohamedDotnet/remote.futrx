import { Activity, Cpu, MessageSquare } from "../../primitives/icons";
import { ComposerOptionDropdown } from "./ComposerOptionDropdown";
import type { ComposerPreferenceActions, ComposerPreferences } from "./preferences";

export function ComposerOptionsRow({
  preferences,
  preferenceActions,
  streaming,
  reasoningEffortOptions,
  serviceTierOptions,
  modeOptions,
}: {
  preferences: ComposerPreferences;
  preferenceActions: ComposerPreferenceActions;
  streaming: boolean;
  reasoningEffortOptions: readonly { value: string; label: string }[];
  serviceTierOptions: readonly { value: string; label: string }[];
  modeOptions: readonly { value: string; label: string }[];
}) {
  return (
    <div class="codex-composer-secondary-controls px-3 pt-1.5 pb-2">
      <div class="flex w-full min-w-0 flex-wrap items-center gap-2">
        {reasoningEffortOptions.length > 0 && (
          <ComposerOptionDropdown
            label="Thinking"
            value={preferences.reasoningEffort}
            options={reasoningEffortOptions}
            disabled={streaming}
            Icon={Activity}
            onChange={preferenceActions.changeReasoningEffort}
          />
        )}

        {serviceTierOptions.length > 0 && (
          <ComposerOptionDropdown
            label="Speed"
            value={preferences.serviceTier}
            options={serviceTierOptions}
            disabled={streaming}
            Icon={Cpu}
            onChange={preferenceActions.changeServiceTier}
          />
        )}

        {modeOptions.length > 0 && (
          <ComposerOptionDropdown
            label="Mode"
            value={preferences.mode}
            options={modeOptions}
            Icon={MessageSquare}
            onChange={preferenceActions.changeMode}
          />
        )}
      </div>
    </div>
  );
}
