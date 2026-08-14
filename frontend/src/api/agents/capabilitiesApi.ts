import { requestJson } from "../apiRequest";
import { API_ROUTES } from "../../config/routes";
import type { AgentCapabilitiesCatalog } from "../../models/agentCapabilities";

export const capabilitiesApi = {
  list: (projectId?: string) => {
    const params = new URLSearchParams();
    if (projectId) params.set("projectId", projectId);
    return requestJson<AgentCapabilitiesCatalog>(
      "GET",
      API_ROUTES.agentCapabilities(params.toString()),
    );
  },
};
