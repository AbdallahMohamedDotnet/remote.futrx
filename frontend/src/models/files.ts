export interface FileNode {
  name: string;
  /** Path relative to the workspace root, forward slashes (used by download URLs). */
  path: string;
  isDir: boolean;
  size?: number;
  modTime?: number;
}

export interface DirListing {
  /** The directory that was listed ("" = workspace root). */
  path: string;
  entries: FileNode[];
  /** True when the directory hit the per-listing entry cap and is partial. */
  truncated: boolean;
}

export interface FileSearchResult {
  entries: FileNode[];
  /** True when the search hit its result/visit cap and is partial. */
  truncated: boolean;
}

/** What the in-app viewer can render inline, mirroring the backend's
 *  workspacefiles mediaTypes. */
export type MediaKind = "image" | "video" | "audio" | "pdf";
