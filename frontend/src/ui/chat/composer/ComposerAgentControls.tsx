import type { ChatProvider, SelectedSkill } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import { ComposerModelPicker } from "./ComposerModelPicker";
import { ProviderToggle } from "./ProviderToggle";
import { SkillPicker } from "./SkillPicker";

export function ComposerAgentControls({
  projectId,
  model,
  provider,
  streaming,
  providerOptions,
  modelOptions,
  selectedSkills,
  onSelectSkill,
  onProviderChange,
  onModelChange,
}: {
  projectId?: string;
  model: string;
  provider: ChatProvider;
  streaming: boolean;
  providerOptions: readonly { value: ChatProvider; label: string }[];
  modelOptions: readonly { value: string; label: string; sub: string }[];
  selectedSkills: SelectedSkill[];
  onSelectSkill: (skill: RegisteredSkill) => void;
  onProviderChange: (provider: ChatProvider) => void;
  onModelChange: (model: string) => void;
}) {
  const selectedCount = selectedSkills.length;
  return (
    <div class="codex-composer-agent-controls flex min-w-0 flex-wrap items-center gap-1">
      <ProviderToggle
        provider={provider}
        options={providerOptions}
        streaming={streaming}
        onChange={onProviderChange}
      />

      <ComposerModelPicker
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
