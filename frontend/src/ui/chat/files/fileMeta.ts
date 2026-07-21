import type { FileNode } from "../../../models/files";

export type FileCategory =
  | "image"
  | "video"
  | "audio"
  | "pdf"
  | "archive"
  | "code"
  | "data"
  | "text";

const EXT_CATEGORY: Record<string, FileCategory> = {
  png: "image", jpg: "image", jpeg: "image", gif: "image", webp: "image",
  svg: "image", avif: "image", bmp: "image", ico: "image", heic: "image",
  mp4: "video", mov: "video", webm: "video", mkv: "video", avi: "video", m4v: "video",
  mp3: "audio", wav: "audio", flac: "audio", ogg: "audio", m4a: "audio", aac: "audio",
  pdf: "pdf",
  zip: "archive", tar: "archive", gz: "archive", tgz: "archive", rar: "archive", "7z": "archive",
  ts: "code", tsx: "code", js: "code", jsx: "code", go: "code", py: "code", rs: "code",
  java: "code", c: "code", cpp: "code", h: "code", css: "code", html: "code", sh: "code", rb: "code",
  json: "data", csv: "data", yaml: "data", yml: "data", xml: "data", toml: "data",
  sql: "data", db: "data", sqlite: "data",
  txt: "text", md: "text", log: "text",
};

export function categorize(name: string): FileCategory {
  const dot = name.lastIndexOf(".");
  if (dot < 0) return "text";
  return EXT_CATEGORY[name.slice(dot + 1).toLowerCase()] ?? "text";
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(value < 10 ? 1 : 0)} ${units[i]}`;
}

/** Stable expand-state key for a folder within a given dir tree. */
export function nodeKey(dir: string, p: string): string {
  return `${dir}:${p}`;
}

export function countFiles(nodes?: FileNode[]): number {
  if (!nodes) return 0;
  let total = 0;
  for (const node of nodes) {
    total += node.isDir ? countFiles(node.children) : 1;
  }
  return total;
}

export function treeStats(nodes: FileNode[]): { files: number; size: number } {
  let files = 0;
  let size = 0;
  const walk = (list: FileNode[]) => {
    for (const node of list) {
      if (node.isDir) {
        if (node.children) walk(node.children);
      } else {
        files++;
        size += node.size ?? 0;
      }
    }
  };
  walk(nodes);
  return { files, size };
}

/** Every folder key in the tree — used by "expand all". */
export function collectFolderKeys(dir: string, nodes: FileNode[], acc: Set<string>): Set<string> {
  for (const node of nodes) {
    if (node.isDir) {
      acc.add(nodeKey(dir, node.path));
      if (node.children) collectFolderKeys(dir, node.children, acc);
    }
  }
  return acc;
}

/**
 * Prune a tree to files whose name matches the query (case-insensitive) plus
 * their ancestor folders, accumulating the folder keys that must be force-open
 * so every match is visible.
 */
export function filterTree(
  dir: string,
  nodes: FileNode[],
  query: string,
  openAcc: Set<string>
): FileNode[] {
  const q = query.trim().toLowerCase();
  if (!q) return nodes;
  const out: FileNode[] = [];
  for (const node of nodes) {
    if (node.isDir) {
      const kids = node.children ? filterTree(dir, node.children, query, openAcc) : [];
      if (kids.length > 0 || node.name.toLowerCase().includes(q)) {
        openAcc.add(nodeKey(dir, node.path));
        out.push({ ...node, children: kids });
      }
    } else if (node.name.toLowerCase().includes(q)) {
      out.push(node);
    }
  }
  return out;
}
