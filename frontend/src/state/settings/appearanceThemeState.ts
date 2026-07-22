import { SYSTEM_LIGHT_MEDIA_QUERY } from "../../config/settings";
import type { AppearanceTheme } from "../../models/settings";

class AppearanceThemeState {
  apply(theme: AppearanceTheme): void {
    if (typeof document === "undefined") return;
    const resolved = this.resolve(theme);
    const root = document.documentElement;
    root.dataset.theme = resolved;
    root.dataset.themeChoice = theme;
    root.classList.toggle("light", resolved === "light");
    root.classList.toggle("dark", resolved === "dark");
    root.style.colorScheme = resolved;
  }

  observeSystemChanges(theme: AppearanceTheme): (() => void) | undefined {
    if (theme !== "system" || typeof window === "undefined") return;

    const query = window.matchMedia(SYSTEM_LIGHT_MEDIA_QUERY);
    const applySystemTheme = () => this.apply("system");
    query.addEventListener("change", applySystemTheme);
    return () => query.removeEventListener("change", applySystemTheme);
  }

  private resolve(theme: AppearanceTheme): "dark" | "light" {
    if (theme === "light" || theme === "dark") return theme;
    if (typeof window === "undefined") return "dark";
    return window.matchMedia(SYSTEM_LIGHT_MEDIA_QUERY).matches ? "light" : "dark";
  }
}

export const appearanceThemeState = new AppearanceThemeState();
