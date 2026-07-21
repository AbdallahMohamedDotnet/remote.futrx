import type {
  ChatMeta,
  ChatMode,
  ChatProvider,
  ReasoningEffort,
  SelectedSkill,
} from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import { useUserSettingsContext } from "../../context/UserSettingsContext";
import { useChatMetaActions } from "./useChatMetaActions";

export function useChatPreferences({
  chat,
  loadedMeta,
  refreshMeta,
}: {
  chat: ChatMeta;
  loadedMeta: ChatMeta | null;
  refreshMeta: () => Promise<void>;
}) {
  const { settings, setChatSettings } = useUserSettingsContext();
  const baseMeta = loadedMeta ?? chat;
  const displayProvider = baseMeta.provider || settings.chat.provider;
  const displayMode = baseMeta.mode || settings.chat.mode;
  const displayMeta: ChatMeta = {
    ...baseMeta,
    provider: displayProvider,
    model: baseMeta.model ?? settings.chat.model,
    mode: displayMode,
    reasoningEffort: baseMeta.reasoningEffort ?? settings.chat.reasoningEffort,
  };
  const selectedSkills = displayMeta.selectedSkills || [];
  const metaActions = useChatMetaActions({ chatId: chat.id, refreshMeta });

  function changeProvider(provider: ChatProvider) {
    if (provider === displayProvider) return;
    metaActions.applyMeta({ provider, model: "", reasoningEffort: "", selectedSkills: [] });
    void setChatSettings({ provider, model: "", reasoningEffort: "" });
  }

  function selectSkill(skill: RegisteredSkill) {
    const next: SelectedSkill = {
      name: skill.name,
      command: skill.command || skill.name,
      provider: skill.provider || displayProvider,
      source: skill.source,
    };
    if (selectedSkills.some((selected) => skillKey(selected, displayProvider) === skillKey(next, displayProvider))) {
      return;
    }
    metaActions.applyMeta({ selectedSkills: [...selectedSkills, next] });
  }

  function removeSelectedSkill(skill: SelectedSkill) {
    const key = skillKey(skill, displayProvider);
    metaActions.applyMeta({
      selectedSkills: selectedSkills.filter((selected) => skillKey(selected, displayProvider) !== key),
    });
  }

  function changeModel(model: string) {
    metaActions.applyMeta({ model });
    void setChatSettings({ model });
  }

  function changeMode(mode: ChatMode) {
    metaActions.applyMeta({ mode });
    void setChatSettings({ mode });
  }

  function changeReasoningEffort(reasoningEffort: ReasoningEffort) {
    metaActions.applyMeta({ reasoningEffort });
    void setChatSettings({ reasoningEffort });
  }

  return {
    displayMeta,
    displayMode,
    selectedSkills,
    changeProvider,
    changeModel,
    changeMode,
    changeReasoningEffort,
    selectSkill,
    removeSelectedSkill,
  };
}

function skillKey(skill: SelectedSkill | RegisteredSkill, defaultProvider: ChatProvider) {
  const provider = skill.provider || defaultProvider;
  const source = skill.source || "";
  const command = (skill.command || skill.name).trim().toLowerCase();
  return `${provider}:${source.toLowerCase()}:${command}`;
}
