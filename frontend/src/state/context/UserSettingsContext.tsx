import type { ComponentChildren } from "preact";
import { createContext } from "preact";
import { useCallback, useContext, useEffect, useMemo, useState } from "preact/hooks";
import { useAuthContext } from "./AuthContext";
import {
  DEFAULT_USER_SETTINGS,
  type AppearanceTheme,
  type ChatSettings,
  type UserSettings,
} from "../../models/settings";
import { settingsApi } from "../../api/settingsApi";

interface UserSettingsContextValue {
  settings: UserSettings;
  loading: boolean;
  saving: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  setTheme: (theme: AppearanceTheme) => Promise<void>;
  setChatSettings: (chat: Partial<ChatSettings>) => Promise<void>;
}

const UserSettingsContext = createContext<UserSettingsContextValue | null>(null);
const systemLightQuery = "(prefers-color-scheme: light)";

export function UserSettingsProvider({ children }: { children: ComponentChildren }) {
  const { googleOk } = useAuthContext();
  const [settings, setSettings] = useState<UserSettings>(DEFAULT_USER_SETTINGS);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!googleOk) {
      setSettings(DEFAULT_USER_SETTINGS);
      setLoading(false);
      setError(null);
      return;
    }

    setLoading(true);
    try {
      setSettings(await settingsApi.get());
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [googleOk]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    applyTheme(settings.appearance.theme);
    if (settings.appearance.theme !== "system" || typeof window === "undefined") return;

    const query = window.matchMedia(systemLightQuery);
    const onChange = () => applyTheme("system");
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, [settings.appearance.theme]);

  const setTheme = useCallback(async (theme: AppearanceTheme) => {
    const previous = settings;
    setSettings({ ...settings, appearance: { ...settings.appearance, theme } });
    setSaving(true);
    try {
      setSettings(await settingsApi.update({ appearance: { theme } }));
      setError(null);
    } catch (e) {
      setSettings(previous);
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }, [settings]);

  const setChatSettings = useCallback(async (chat: Partial<ChatSettings>) => {
    const previous = settings;
    setSettings({ ...settings, chat: { ...settings.chat, ...chat } });
    setSaving(true);
    try {
      setSettings(await settingsApi.update({ chat }));
      setError(null);
    } catch (e) {
      setSettings(previous);
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }, [settings]);

  const value = useMemo<UserSettingsContextValue>(() => ({
    settings,
    loading,
    saving,
    error,
    refresh,
    setTheme,
    setChatSettings,
  }), [settings, loading, saving, error, refresh, setTheme, setChatSettings]);

  return (
    <UserSettingsContext.Provider value={value}>
      {children}
    </UserSettingsContext.Provider>
  );
}

export function useUserSettingsContext(): UserSettingsContextValue {
  const value = useContext(UserSettingsContext);
  if (!value) throw new Error("useUserSettingsContext must be used inside UserSettingsProvider");
  return value;
}

function applyTheme(theme: AppearanceTheme) {
  if (typeof document === "undefined") return;
  const resolved = resolveTheme(theme);
  const root = document.documentElement;
  root.dataset.theme = resolved;
  root.dataset.themeChoice = theme;
  root.classList.toggle("light", resolved === "light");
  root.classList.toggle("dark", resolved === "dark");
  root.style.colorScheme = resolved;
}

function resolveTheme(theme: AppearanceTheme): "dark" | "light" {
  if (theme === "light" || theme === "dark") return theme;
  if (typeof window === "undefined") return "dark";
  return window.matchMedia(systemLightQuery).matches ? "light" : "dark";
}
