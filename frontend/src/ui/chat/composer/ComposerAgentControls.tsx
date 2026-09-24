import type { ChatProvider, SelectedSkill } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import type { AgentAuthAccountsSnapshot } from "../../../models/auth";
import type {
  ComposerModelOption,
  ComposerProviderOption,
} from "../../../models/agentCapabilities";
import { ComposerAgentPicker } from "./ComposerAgentPicker";
import { ComposerAccountPicker } from "./ComposerAccountPicker";
import { SkillPicker } from "./SkillPicker";

export function ComposerAgentControls({
  projectId,
  model,
  provider,
  accountId,
  accounts,
  streaming,
  providerOptions,
  modelOptions,
  modelsLoading,
  modelsRefreshing,
  modelError,
  selectedSkills,
  providerLabel,
  skillsEnabled,
  onSelectSkill,
  onAgentChange,
  onAccountChange,
  onRefreshModels,
}: {
  projectId?: string;
  model: string;
  provider: ChatProvider;
  accountId: string;
  accounts?: AgentAuthAccountsSnapshot;
  streaming: boolean;
  providerOptions: readonly ComposerProviderOption[];
  modelOptions: readonly ComposerModelOption[];
  modelsLoading: boolean;
  modelsRefreshing: boolean;
  modelError: string;
  selectedSkills: SelectedSkill[];
  providerLabel: string;
  skillsEnabled: boolean;
  onSelectSkill: (skill: RegisteredSkill) => void;
  onAgentChange: (provider: ChatProvider, model: string) => void;
  onAccountChange: (accountId: string) => void;
  onRefreshModels: () => Promise<void>;
}) {
  const selectedCount = selectedSkills.length;
  return (
    <div class="codex-composer-agent-controls flex min-w-0 flex-wrap items-center gap-1">
      {accounts && accounts.items.length > 0 && (
        <ComposerAccountPicker
          accountId={accountId}
          accounts={accounts}
          streaming={streaming}
          onChange={onAccountChange}
        />
      )}
      <ComposerAgentPicker
        provider={provider}
        model={model}
        streaming={streaming}
        providerOptions={providerOptions}
        modelOptions={modelOptions}
        loading={modelsLoading}
        refreshing={modelsRefreshing}
        error={modelError}
        onChange={onAgentChange}
        onRefresh={onRefreshModels}
      />

      {skillsEnabled && (
        <SkillPicker
          provider={provider}
          providerLabel={providerLabel}
          projectId={projectId}
          selectedCount={selectedCount}
          onSelect={(skill) => onSelectSkill(skill)}
        />
      )}
    </div>
  );
}
