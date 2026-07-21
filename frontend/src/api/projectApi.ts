import { requestJson } from "./apiRequest";
import { projectBrowserApi } from "./project/projectBrowserApi";
import { projectContainerApi } from "./project/projectContainerApi";
import { projectSecretsApi } from "./project/projectSecretsApi";
import type { ProjectMeta } from "../models/project";
import { API_ROUTES } from "../config/routes";

export const projectApi = {
  list: () => requestJson<ProjectMeta[]>("GET", API_ROUTES.projects.collection),
  create: (name: string) =>
    requestJson<ProjectMeta>("POST", API_ROUTES.projects.collection, { name }),
  get: (id: string) =>
    requestJson<ProjectMeta>("GET", API_ROUTES.projects.item(id)),
  update: (id: string, body: { name?: string }) =>
    requestJson<ProjectMeta>("PATCH", API_ROUTES.projects.item(id), body),
  reorder: (ids: string[]) =>
    requestJson<ProjectMeta[]>("POST", API_ROUTES.projects.reorder, { ids }),
  delete: (id: string) =>
    requestJson<{ ok: boolean }>("DELETE", API_ROUTES.projects.item(id)),
  start: projectContainerApi.start,
  stop: projectContainerApi.stop,
  containerInfo: projectContainerApi.containerInfo,
  repairNetwork: projectContainerApi.repairNetwork,
  listApps: projectBrowserApi.listApps,
  agentBrowserStatus: projectBrowserApi.agentBrowserStatus,
  startAgentBrowser: projectBrowserApi.startAgentBrowser,
  stopAgentBrowser: projectBrowserApi.stopAgentBrowser,
  listSecrets: projectSecretsApi.listSecrets,
  setSecret: projectSecretsApi.setSecret,
  deleteSecret: projectSecretsApi.deleteSecret,
  listAccess: (id: string) =>
    requestJson<string[]>("GET", API_ROUTES.projects.access(id)),
  addAccess: (id: string, email: string) =>
    requestJson<{ email: string }>(
      "POST",
      API_ROUTES.projects.access(id),
      { email }
    ),
  removeAccess: (id: string, email: string) =>
    requestJson<{ ok: boolean }>(
      "DELETE",
      API_ROUTES.projects.accessMember(id, email)
    ),
};
