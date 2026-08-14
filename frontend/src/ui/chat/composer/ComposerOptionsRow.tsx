import {
  MODE_OPTIONS,
  reasoningEffortOptionsForProvider,
  serviceTierOptionsForProvider,
} from "../../../config/chat";
import { Activity, Cpu, MessageSquare } from "../../primitives/icons";
import { ComposerOptionDropdown } from "./ComposerOptionDropdown";
import type { ComposerPreferenceActions, ComposerPreferences } from "./preferences";

export function ComposerOptionsRow({
  preferences,
  preferenceActions,
  streaming,
}: {
  preferences: ComposerPreferences;
  preferenceActions: ComposerPreferenceActions;
  streaming: boolean;
}) {
  const reasoningEffortOptions = reasoningEffortOptionsForProvider(preferences.provider);
  const serviceTierOptions = serviceTierOptionsForProvider(preferences.provider);

  return (
    <div class="codex-composer-secondary-controls flex min-w-0 flex-wrap items-center gap-1">
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

      <ComposerOptionDropdown
        label="Mode"
        value={preferences.mode}
        options={MODE_OPTIONS}
        Icon={MessageSquare}
        onChange={preferenceActions.changeMode}
      />
    </div>
  );
}
