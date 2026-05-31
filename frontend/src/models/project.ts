export type ProjectStatus =
  | ""
  | "provisioning"
  | "running"
  | "stopped"
  | "error"
  | "missing";

export interface ProjectMeta {
  id: string;
  name: string;
  slug: string;
  cwd: string;
  containerName: string;
  status: ProjectStatus;
  errorMsg?: string;
  createdAt: number;
  updatedAt: number;
}

export type ProjectContainerInfo = unknown;
