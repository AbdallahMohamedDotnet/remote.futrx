import type { ChatProvider } from "../../../models/chat";
import { modelOptionsForProvider } from "../../../state/chat/usage";
import { ComposerModelPicker } from "./ComposerModelPicker";
import { ProviderToggle } from "./ProviderToggle";
import { SkillPicker } from "./SkillPicker";

export function ComposerToolbar({
  projectId,
  model,
  provider,
  streaming,
  onInsertSkill,
  onProviderChange,
  onModelChange,
}: {
  projectId?: string;
  model: string;
  provider: ChatProvider;
  streaming: boolean;
  onInsertSkill: (skillName: string) => void;
  onProviderChange: (provider: ChatProvider) => void;
  onModelChange: (model: string) => void;
}) {
  const modelOptions = modelOptionsForProvider(provider);
  return (
    <div class="codex-composer-primary-controls px-3 pt-2 pb-1">
      <div class="flex w-full min-w-0 flex-wrap items-center gap-1.5">
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

        <SkillPicker
          provider={provider}
          projectId={projectId}
          onSelect={(skill) => onInsertSkill(skill.name)}
        />
      </div>
    </div>
  );
}
