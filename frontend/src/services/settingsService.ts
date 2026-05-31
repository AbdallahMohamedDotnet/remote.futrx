import { json } from "../api/http";
import {
  DEFAULT_USER_SETTINGS,
  type AppearanceTheme,
  type SettingsSaveResult,
  type UpdateUserSettingsInput,
  type UserSettings,
} from "../models/settings";

const themes = new Set<AppearanceTheme>(["system", "dark", "light"]);

export const settingsService = {
  get: async () => normalize(await json<UserSettings>("GET", "/api/me/settings")),
  update: async (body: UpdateUserSettingsInput): Promise<SettingsSaveResult> => {
    const raw = await json<SettingsSaveResult>("PATCH", "/api/me/settings", body);
    return { ...raw, ...normalize(raw) } as SettingsSaveResult;
  },
};

function normalize(settings: UserSettings): UserSettings {
  const theme = settings?.appearance?.theme;
  return {
    ...DEFAULT_USER_SETTINGS,
    ...settings,
    appearance: {
      ...DEFAULT_USER_SETTINGS.appearance,
      ...settings?.appearance,
      theme: themes.has(theme) ? theme : DEFAULT_USER_SETTINGS.appearance.theme,
    },
  };
}
