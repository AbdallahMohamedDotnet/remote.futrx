import { json } from "../api/http";
import type { ChatProvider } from "../models/chat";
import type { RegisteredSkill } from "../models/skill";

export const skillService = {
  list: (provider: ChatProvider) =>
    json<RegisteredSkill[]>("GET", `/api/skills?provider=${encodeURIComponent(provider)}`),
};

