import type { ChatProvider } from "./chat";

export interface RegisteredSkill {
  name: string;
  description?: string;
  provider: ChatProvider;
  source?: "user" | "system" | "plugin" | string;
}

