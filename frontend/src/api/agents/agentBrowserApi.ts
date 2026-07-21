import { requestJson } from "../apiRequest";
import type { AgentBrowserInfo } from "../../models/project";
import { API_ROUTES } from "../../config/routes";

export const agentBrowserApi = {
  fetchAgentBrowserStatus: (id: string) =>
    requestJson<AgentBrowserInfo>(
      "GET",
      API_ROUTES.projects.agentBrowser(id)
    ),

  startAgentBrowser: (id: string) =>
    requestJson<AgentBrowserInfo>(
      "POST",
      API_ROUTES.projects.startAgentBrowser(id),
      {}
    ),

  stopAgentBrowser: (id: string, scope?: "view") =>
    requestJson<AgentBrowserInfo | { status: "stopped" }>(
      "DELETE",
      API_ROUTES.projects.agentBrowser(id, scope)
    ),
};
