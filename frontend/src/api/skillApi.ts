import { json } from "../transport/http";
import type { ChatProvider } from "../models/chat";
import type { RegisteredSkill } from "../models/skill";

export const skillApi = {
  list: (provider: ChatProvider, projectId?: string) => {
    const params = new URLSearchParams({ provider });
    if (projectId) params.set("projectId", projectId);
    return json<RegisteredSkill[]>("GET", `/api/skills?${params.toString()}`);
  },
};
