import { requestJson } from "../apiRequest";
import type { ChatProvider } from "../../models/chat";
import type { RegisteredSkill } from "../../models/skill";
import { API_ROUTES } from "../../config/routes";

export const skillApi = {
  list: (provider: ChatProvider, projectId?: string) => {
    const params = new URLSearchParams({ provider });
    if (projectId) params.set("projectId", projectId);
    return requestJson<RegisteredSkill[]>(
      "GET",
      API_ROUTES.skills(params.toString())
    );
  },
};
