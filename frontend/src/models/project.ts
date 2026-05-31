export type ProjectStatus =
  | ""
  | "provisioning"
  | "running"
  | "stopped"
  | "error"
  | "missing";

export type ContainerState =
  | "RUNNING"
  | "STOPPED"
  | "FROZEN"
  | "MISSING"
  | "UNKNOWN";

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

export interface WorkspaceInfo {
  hostSource: string;
  containerPath: string;
}

export interface ResourceInfo {
  processes?: number;
  diskUsageBytes?: number;
  memoryCurrentBytes?: number;
  memoryPeakBytes?: number;
  memoryTotalBytes?: number;
  swapCurrentBytes?: number;
  cpuUsageSeconds?: number;
}

export interface NetworkInterface {
  name: string;
  state?: string;
  type?: string;
  hostName?: string;
  macAddress?: string;
  mtu?: number;
  addresses?: string[];
  bytesReceived?: number;
  bytesSent?: number;
}

export interface OSInfo {
  prettyName?: string;
  kernel?: string;
  uptimeSec?: number;
  cpuCount?: number;
  hostname?: string;
}

export interface DiskUsage {
  mountPath: string;
  filesystem?: string;
  totalBytes?: number;
  usedBytes?: number;
  availBytes?: number;
}

export interface ContainerLimits {
  cpu?: string;
  memory?: string;
  disk?: string;
}

export interface ClaudeContainerStatus {
  installed: boolean;
  version?: string;
  claudeMdInstalled: boolean;
  claudeMdInSync: boolean;
}

export interface CodexContainerStatus {
  installed: boolean;
  version?: string;
}

export interface AuthBundleFileStatus {
  hostPath: string;
  containerPath: string;
  hostExists: boolean;
  hostMtime?: number;
  containerExists: boolean;
  containerMtime?: number;
  hostNewer: boolean;
  containerNewer: boolean;
}

export interface AuthBundleStatus {
  name: string;
  files: AuthBundleFileStatus[];
}

export interface ProjectContainerInfo {
  name: string;
  state: ContainerState;
  bootAutostart: boolean;
  image?: string;
  type?: string;
  architecture?: string;
  pid?: number;
  createdAt?: string;
  lastUsedAt?: string;
  workspace?: WorkspaceInfo;
  resources?: ResourceInfo;
  network?: NetworkInterface[];
  os?: OSInfo;
  disks?: DiskUsage[];
  limits?: ContainerLimits;
  claude: ClaudeContainerStatus;
  codex: CodexContainerStatus;
  authBundles: AuthBundleStatus[];
}

export interface ProjectSecret {
  key: string;
  value: string;
  updatedAt: number;
}

export interface ProjectDBViewer {
  url: string;
  port: number;
  containerName: string;
}
