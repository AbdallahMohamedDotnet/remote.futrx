import type {
  ApprovalPolicy,
  ChatMode,
  ChatProvider,
  ReasoningEffort,
  SandboxPolicy,
  ServiceTier,
} from "./chat";

export type AppearanceTheme = "system" | "dark" | "light";

export interface AppearanceSettings {
  theme: AppearanceTheme;
}

export interface ChatSettings {
  provider: ChatProvider;
  model: string;
  mode: ChatMode;
  reasoningEffort: ReasoningEffort;
  serviceTier: ServiceTier;
  approvalPolicy: ApprovalPolicy;
  sandboxPolicy: SandboxPolicy;
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
