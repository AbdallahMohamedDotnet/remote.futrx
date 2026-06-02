import { json } from "../api/http";
import type {
  ContainerApp,
  ProjectContainerInfo,
  ProjectMeta,
  ProjectSecret,
} from "../models/project";

export const projectService = {
  list: () => json<ProjectMeta[]>("GET", "/api/projects"),
  create: (name: string) => json<ProjectMeta>("POST", "/api/projects", { name }),
  get: (id: string) => json<ProjectMeta>("GET", `/api/projects/${encodeURIComponent(id)}`),
  update: (id: string, body: { name?: string }) =>
    json<ProjectMeta>("PATCH", `/api/projects/${encodeURIComponent(id)}`, body),
  delete: (id: string) =>
    json<{ ok: boolean }>("DELETE", `/api/projects/${encodeURIComponent(id)}`),
  start: (id: string) =>
    json<ProjectMeta>("POST", `/api/projects/${encodeURIComponent(id)}/start`, {}),
  stop: (id: string) =>
    json<ProjectMeta>("POST", `/api/projects/${encodeURIComponent(id)}/stop`, {}),
  containerInfo: (id: string) =>
    json<ProjectContainerInfo>("GET", `/api/projects/${encodeURIComponent(id)}/container`),
  listApps: (id: string) =>
    json<ContainerApp[]>("GET", `/api/projects/${encodeURIComponent(id)}/apps`),
  listSecrets: (id: string) =>
    json<ProjectSecret[]>("GET", `/api/projects/${encodeURIComponent(id)}/secrets`),
  setSecret: (id: string, key: string, value: string) =>
    json<ProjectSecret>(
      "PUT",
      `/api/projects/${encodeURIComponent(id)}/secrets/${encodeURIComponent(key)}`,
      { value }
    ),
  deleteSecret: (id: string, key: string) =>
    json<{ ok: boolean }>(
      "DELETE",
      `/api/projects/${encodeURIComponent(id)}/secrets/${encodeURIComponent(key)}`
    ),
  listAccess: (id: string) =>
    json<string[]>("GET", `/api/projects/${encodeURIComponent(id)}/access`),
  addAccess: (id: string, email: string) =>
    json<{ email: string }>(
      "POST",
      `/api/projects/${encodeURIComponent(id)}/access`,
      { email }
    ),
  removeAccess: (id: string, email: string) =>
    json<{ ok: boolean }>(
      "DELETE",
      `/api/projects/${encodeURIComponent(id)}/access/${encodeURIComponent(email)}`
    ),
};
