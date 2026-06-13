export interface FileNode {
  name: string;
  /** Path relative to the dir root, forward slashes (used by download URLs). */
  path: string;
  isDir: boolean;
  size?: number;
  modTime?: number;
  children?: FileNode[];
}

export interface FileTree {
  /** ".uploads" or ".media" */
  dir: string;
  exists: boolean;
  children: FileNode[];
}

export interface FileTreeResponse {
  trees: FileTree[];
  /** True when the listing hit the node/depth cap and is partial. */
  truncated: boolean;
}
