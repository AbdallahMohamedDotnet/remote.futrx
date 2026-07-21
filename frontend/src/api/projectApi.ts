import { requestJson } from "./apiRequest";
import type {
  AgentBrowserInfo,
  ContainerApp,
  ProjectContainerInfo,
  ProjectMeta,
  ProjectSecret,
} from "../models/project";
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
  start: (id: string) =>
    requestJson<ProjectMeta>("POST", API_ROUTES.projects.start(id), {}),
  stop: (id: string) =>
    requestJson<ProjectMeta>("POST", API_ROUTES.projects.stop(id), {}),
  containerInfo: (id: string) =>
    requestJson<ProjectContainerInfo>("GET", API_ROUTES.projects.container(id)),
  repairNetwork: (id: string) =>
    requestJson<ProjectContainerInfo>(
      "POST",
      API_ROUTES.projects.repairNetwork(id),
      {}
    ),
  listApps: (id: string) =>
    requestJson<ContainerApp[]>("GET", API_ROUTES.projects.apps(id)),
  agentBrowserStatus: (id: string) =>
    requestJson<AgentBrowserInfo>("GET", API_ROUTES.projects.agentBrowser(id)),
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
  listSecrets: (id: string) =>
    requestJson<ProjectSecret[]>("GET", API_ROUTES.projects.secrets(id)),
  setSecret: (id: string, key: string, value: string) =>
    requestJson<ProjectSecret>(
      "PUT",
      API_ROUTES.projects.secret(id, key),
      { value }
    ),
  deleteSecret: (id: string, key: string) =>
    requestJson<{ ok: boolean }>(
      "DELETE",
      API_ROUTES.projects.secret(id, key)
    ),
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
