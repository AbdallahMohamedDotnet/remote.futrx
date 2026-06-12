export interface WorkspaceFile {
  name: string;
  size: number;
  modTime: number;
}

export interface WorkspaceDirListing {
  dir: string;
  exists: boolean;
  files: WorkspaceFile[];
}

export interface WorkspaceFilesResponse {
  dirs: WorkspaceDirListing[];
}
