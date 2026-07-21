import { requestJson } from "./apiRequest";
import type { ChatMode, ChatProvider, ReasoningEffort } from "../models/chat";
import {
  DEFAULT_USER_SETTINGS,
  type AppearanceTheme,
  type UpdateUserSettingsInput,
  type UserSettings,
} from "../models/settings";
import { API_ROUTES } from "../config/routes";

const themes = new Set<AppearanceTheme>(["system", "dark", "light"]);
const providers = new Set<ChatProvider>(["claude", "codex"]);
const modes = new Set<ChatMode>(["chat", "plan", "code", "review", "debug", "full-auto"]);
const reasoningEfforts = new Set<ReasoningEffort>(["", "low", "medium", "high", "xhigh"]);

export const settingsApi = {
  get: async () =>
    normalize(await requestJson<UserSettings>("GET", API_ROUTES.settings)),
  update: async (body: UpdateUserSettingsInput) =>
    normalize(await requestJson<UserSettings>("PATCH", API_ROUTES.settings, body)),
};

function normalize(settings: UserSettings): UserSettings {
  const theme = settings?.appearance?.theme;
  const provider = settings?.chat?.provider;
  const mode = settings?.chat?.mode;
  const reasoningEffort = settings?.chat?.reasoningEffort;
  return {
    ...DEFAULT_USER_SETTINGS,
    ...settings,
    appearance: {
      ...DEFAULT_USER_SETTINGS.appearance,
      ...settings?.appearance,
      theme: themes.has(theme) ? theme : DEFAULT_USER_SETTINGS.appearance.theme,
    },
    chat: {
      ...DEFAULT_USER_SETTINGS.chat,
      ...settings?.chat,
      provider: providers.has(provider) ? provider : DEFAULT_USER_SETTINGS.chat.provider,
      model: typeof settings?.chat?.model === "string" ? settings.chat.model : DEFAULT_USER_SETTINGS.chat.model,
      mode: modes.has(mode) ? mode : DEFAULT_USER_SETTINGS.chat.mode,
      reasoningEffort: reasoningEfforts.has(reasoningEffort)
        ? reasoningEffort
        : DEFAULT_USER_SETTINGS.chat.reasoningEffort,
    },
  };
}
