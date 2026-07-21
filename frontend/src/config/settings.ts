import type { ChatMode, ChatProvider, ReasoningEffort } from "../models/chat";
import type { AppearanceTheme, UserSettings } from "../models/settings";

export const DEFAULT_USER_SETTINGS: UserSettings = {
  appearance: { theme: "system" },
  chat: { provider: "codex", model: "", mode: "code", reasoningEffort: "" },
};

export const VALID_APPEARANCE_THEMES = new Set<AppearanceTheme>([
  "system",
  "dark",
  "light",
]);
export const VALID_CHAT_PROVIDERS = new Set<ChatProvider>(["claude", "codex"]);
export const VALID_CHAT_MODES = new Set<ChatMode>([
  "chat",
  "plan",
  "code",
  "review",
  "debug",
  "full-auto",
]);
export const VALID_REASONING_EFFORTS = new Set<ReasoningEffort>([
  "",
  "low",
  "medium",
  "high",
  "xhigh",
]);

export const SYSTEM_LIGHT_MEDIA_QUERY = "(prefers-color-scheme: light)";
