import { requestJson } from "../transport/http";
import type {
  AgentBrowserInfo,
  ContainerApp,
  ProjectContainerInfo,
  ProjectMeta,
  ProjectSecret,
} from "../models/project";

export const projectApi = {
  list: () => requestJson<ProjectMeta[]>("GET", "/api/projects"),
  create: (name: string) => requestJson<ProjectMeta>("POST", "/api/projects", { name }),
  get: (id: string) => requestJson<ProjectMeta>("GET", `/api/projects/${encodeURIComponent(id)}`),
  update: (id: string, body: { name?: string }) =>
    requestJson<ProjectMeta>("PATCH", `/api/projects/${encodeURIComponent(id)}`, body),
  reorder: (ids: string[]) =>
    requestJson<ProjectMeta[]>("POST", "/api/projects/reorder", { ids }),
  delete: (id: string) =>
    requestJson<{ ok: boolean }>("DELETE", `/api/projects/${encodeURIComponent(id)}`),
  start: (id: string) =>
    requestJson<ProjectMeta>("POST", `/api/projects/${encodeURIComponent(id)}/start`, {}),
  stop: (id: string) =>
    requestJson<ProjectMeta>("POST", `/api/projects/${encodeURIComponent(id)}/stop`, {}),
  containerInfo: (id: string) =>
    requestJson<ProjectContainerInfo>("GET", `/api/projects/${encodeURIComponent(id)}/container`),
  repairNetwork: (id: string) =>
    requestJson<ProjectContainerInfo>("POST", `/api/projects/${encodeURIComponent(id)}/repair-network`, {}),
  listApps: (id: string) =>
    requestJson<ContainerApp[]>("GET", `/api/projects/${encodeURIComponent(id)}/apps`),
  agentBrowserStatus: (id: string) =>
    requestJson<AgentBrowserInfo>("GET", `/api/projects/${encodeURIComponent(id)}/agent-browser`),
  startAgentBrowser: (id: string) =>
    requestJson<AgentBrowserInfo>("POST", `/api/projects/${encodeURIComponent(id)}/agent-browser/start`, {}),
  stopAgentBrowser: (id: string, scope?: "view") => {
    const suffix = scope ? `?scope=${encodeURIComponent(scope)}` : "";
    return requestJson<AgentBrowserInfo | { status: "stopped" }>(
      "DELETE",
      `/api/projects/${encodeURIComponent(id)}/agent-browser${suffix}`
    );
  },
  listSecrets: (id: string) =>
    requestJson<ProjectSecret[]>("GET", `/api/projects/${encodeURIComponent(id)}/secrets`),
  setSecret: (id: string, key: string, value: string) =>
    requestJson<ProjectSecret>(
      "PUT",
      `/api/projects/${encodeURIComponent(id)}/secrets/${encodeURIComponent(key)}`,
      { value }
    ),
  deleteSecret: (id: string, key: string) =>
    requestJson<{ ok: boolean }>(
      "DELETE",
      `/api/projects/${encodeURIComponent(id)}/secrets/${encodeURIComponent(key)}`
    ),
  listAccess: (id: string) =>
    requestJson<string[]>("GET", `/api/projects/${encodeURIComponent(id)}/access`),
  addAccess: (id: string, email: string) =>
    requestJson<{ email: string }>(
      "POST",
      `/api/projects/${encodeURIComponent(id)}/access`,
      { email }
    ),
  removeAccess: (id: string, email: string) =>
    requestJson<{ ok: boolean }>(
      "DELETE",
      `/api/projects/${encodeURIComponent(id)}/access/${encodeURIComponent(email)}`
    ),
};
