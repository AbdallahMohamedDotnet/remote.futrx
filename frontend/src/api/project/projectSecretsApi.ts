import { requestJson } from "../apiRequest";
import type { ProjectSecret } from "../../models/project";
import { API_ROUTES } from "../../config/routes";

export const projectSecretsApi = {
  listSecrets: (id: string) =>
    requestJson<ProjectSecret[]>("GET", API_ROUTES.projects.secrets(id)),

  setSecret: (id: string, key: string, value: string) =>
    requestJson<ProjectSecret>("PUT", API_ROUTES.projects.secret(id, key), {
      value,
    }),

  deleteSecret: (id: string, key: string) =>
    requestJson<{ ok: boolean }>(
      "DELETE",
      API_ROUTES.projects.secret(id, key)
    ),
};
