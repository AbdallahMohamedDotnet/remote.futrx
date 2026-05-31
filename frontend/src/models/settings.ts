export type AppearanceTheme = "system" | "dark" | "light";

export interface AppearanceSettings {
  theme: AppearanceTheme;
}

export interface UserSettings {
  appearance: AppearanceSettings;
  updatedAt?: number;
}

export interface UpdateUserSettingsInput {
  appearance?: Partial<AppearanceSettings>;
}

export const DEFAULT_USER_SETTINGS: UserSettings = {
  appearance: { theme: "system" },
};
