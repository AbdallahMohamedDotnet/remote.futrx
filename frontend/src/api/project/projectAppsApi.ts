import { requestJson } from "../apiRequest";
import type { ContainerApp } from "../../models/project";
import { API_ROUTES } from "../../config/routes";

export const projectAppsApi = {
  listApps: (id: string) =>
    requestJson<ContainerApp[]>("GET", API_ROUTES.projects.apps(id)),
};
