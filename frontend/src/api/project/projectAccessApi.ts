import { requestJson } from "../apiRequest";
import { API_ROUTES } from "../../config/routes";

export const projectAccessApi = {
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
