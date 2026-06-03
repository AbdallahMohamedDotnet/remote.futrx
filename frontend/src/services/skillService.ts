import { json } from "../api/http";
import type { ChatProvider } from "../models/chat";
import type { RegisteredSkill } from "../models/skill";

export const skillService = {
  list: (provider: ChatProvider, projectId?: string) => {
    const params = new URLSearchParams({ provider });
    if (projectId) params.set("projectId", projectId);
    return json<RegisteredSkill[]>("GET", `/api/skills?${params.toString()}`);
  },
};
