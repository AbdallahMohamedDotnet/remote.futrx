import type { ChatMode, ChatProvider, ReasoningEffort } from "./chat";

export type AppearanceTheme = "system" | "dark" | "light";

export interface AppearanceSettings {
  theme: AppearanceTheme;
}

export interface ChatSettings {
  provider: ChatProvider;
  model: string;
  mode: ChatMode;
  reasoningEffort: ReasoningEffort;
}

export interface UserSettings {
  appearance: AppearanceSettings;
  chat: ChatSettings;
  updatedAt?: number;
}

export interface UpdateUserSettingsInput {
  appearance?: Partial<AppearanceSettings>;
  chat?: Partial<ChatSettings>;
}

export const DEFAULT_USER_SETTINGS: UserSettings = {
  appearance: { theme: "system" },
  chat: { provider: "codex", model: "", mode: "code", reasoningEffort: "" },
};
