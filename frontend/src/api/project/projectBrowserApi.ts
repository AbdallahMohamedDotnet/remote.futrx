import { requestJson } from "../apiRequest";
import type { AgentBrowserInfo, ContainerApp } from "../../models/project";
import { API_ROUTES } from "../../config/routes";

export const projectBrowserApi = {
  listApps: (id: string) =>
    requestJson<ContainerApp[]>("GET", API_ROUTES.projects.apps(id)),

  agentBrowserStatus: (id: string) =>
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
