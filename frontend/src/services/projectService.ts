import { json } from "../api/http";
import type { ProjectMeta } from "../models/project";

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
};
