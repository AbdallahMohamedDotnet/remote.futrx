import { Activity, Cpu, MessageSquare } from "../../primitives/icons";
import { ComposerOptionDropdown } from "./ComposerOptionDropdown";
import type { ComposerPreferenceActions, ComposerPreferences } from "./preferences";

export function ComposerExecutionControls({
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
  const selectedModeAvailable = modeOptions.some(
    (option) => option.value === preferences.mode,
  );
  const replacementMode = modeOptions.find((option) => option.value === "default")
    ?? modeOptions[0]
    ?? { value: "default", label: "Default" };
  const unsupportedMode = !!preferences.mode
    && preferences.mode !== "default"
    && !selectedModeAvailable;

  return (
    <div class="codex-composer-execution-controls flex min-w-0 flex-wrap items-center gap-1">
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

      {unsupportedMode && (
        <button
          type="button"
          disabled={streaming}
          onClick={() => preferenceActions.changeMode(replacementMode.value)}
          class="rounded-full border border-amber-400/30 bg-amber-400/10 px-2.5 py-1 text-[11px] font-semibold text-amber-200 disabled:opacity-50"
          title={`${preferences.mode} mode is unavailable for this agent`}
        >
          Switch {preferences.mode} to {replacementMode.label}
        </button>
      )}

      {modeOptions.length > 1 && !unsupportedMode && (
        <ComposerOptionDropdown
          label="Mode"
          value={preferences.mode}
          options={modeOptions}
          disabled={streaming}
          Icon={MessageSquare}
          onChange={preferenceActions.changeMode}
        />
      )}
    </div>
  );
}
