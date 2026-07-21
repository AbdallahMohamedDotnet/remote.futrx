import type { ChatMode, ChatProvider, ReasoningEffort, ServiceTier } from "../models/chat";
import type { AppearanceTheme, UserSettings } from "../models/settings";

export const DEFAULT_USER_SETTINGS: UserSettings = {
  appearance: { theme: "system" },
  chat: { provider: "codex", model: "", mode: "code", reasoningEffort: "", serviceTier: "" },
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
  "none",
  "minimal",
  "low",
  "medium",
  "high",
  "xhigh",
  "max",
  "ultra",
]);
export const VALID_SERVICE_TIERS = new Set<ServiceTier>(["", "default", "priority", "fast"]);

export const SYSTEM_LIGHT_MEDIA_QUERY = "(prefers-color-scheme: light)";
