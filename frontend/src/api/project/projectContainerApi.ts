import { requestJson } from "../apiRequest";
import type {
  ContainerLimits,
  ProjectContainerInfo,
  ProjectMeta,
} from "../../models/project";
import { API_ROUTES } from "../../config/routes";

export const projectContainerApi = {
  start: (id: string) =>
    requestJson<ProjectMeta>("POST", API_ROUTES.projects.start(id), {}),

  stop: (id: string) =>
    requestJson<ProjectMeta>("POST", API_ROUTES.projects.stop(id), {}),

  restart: (id: string) =>
    requestJson<ProjectMeta>("POST", API_ROUTES.projects.restart(id), {}),

  fetchContainerInfo: (id: string) =>
    requestJson<ProjectContainerInfo>(
      "GET",
      API_ROUTES.projects.container(id)
    ),

  setContainerLimits: (id: string, limits: ContainerLimits) =>
    requestJson<ProjectContainerInfo>(
      "PUT",
      API_ROUTES.projects.limits(id),
      limits
    ),

  repairNetwork: (id: string) =>
    requestJson<ProjectContainerInfo>(
      "POST",
      API_ROUTES.projects.repairNetwork(id),
      {}
    ),
};
