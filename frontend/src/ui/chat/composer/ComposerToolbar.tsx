import type { ChatProvider, SelectedSkill } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import { modelOptionsForProvider } from "../../../config/chat";
import { ComposerModelPicker } from "./ComposerModelPicker";
import { ProviderToggle } from "./ProviderToggle";
import { SkillPicker } from "./SkillPicker";

export function ComposerToolbar({
  projectId,
  model,
  provider,
  streaming,
  selectedSkills,
  onSelectSkill,
  onProviderChange,
  onModelChange,
}: {
  projectId?: string;
  model: string;
  provider: ChatProvider;
  streaming: boolean;
  selectedSkills: SelectedSkill[];
  onSelectSkill: (skill: RegisteredSkill) => void;
  onProviderChange: (provider: ChatProvider) => void;
  onModelChange: (model: string) => void;
}) {
  const modelOptions = modelOptionsForProvider(provider);
  const selectedCount = selectedSkills.length;
  return (
    <div class="codex-composer-primary-controls flex min-w-0 flex-wrap items-center gap-1">
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
        selectedCount={selectedCount}
        onSelect={(skill) => onSelectSkill(skill)}
      />
    </div>
  );
}
