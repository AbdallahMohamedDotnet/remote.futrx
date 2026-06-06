export const defaultWorkspacePath = "/opt/remote.futrx.dev";

const ideBaseUrl = "https://code.remote.futrx.dev/";
const containerWorkspacePath = "/workspace";
const workspaceSegment = "/workspace";

export function buildIdeUrl(folderPath: string, filePath?: string): string {
  const url = new URL(ideBaseUrl);
  url.searchParams.set("folder", normalizeAbsolutePath(folderPath) || defaultWorkspacePath);
  if (filePath) url.searchParams.set("file", normalizeAbsolutePath(filePath));
  return url.toString();
}

export function internalPathIdeUrl(href: string, context: IdeLinkContext = {}): string | null {
  const path = normalizeAbsolutePath(stripPathSuffix(href));
  if (!path) return null;
  if (!isContainerWorkspacePath(path) && !isHostWorkspacePath(path)) return null;

  if (context.chatId) {
    const params = new URLSearchParams({ path });
    return `/api/chats/${encodeURIComponent(context.chatId)}/ide-open?${params.toString()}`;
  }

  const workspaceRoot = workspaceRootFromCwd(context.cwd);
  if (isContainerWorkspacePath(path)) {
    if (!workspaceRoot) return null;
    const rel = path.slice(containerWorkspacePath.length).replace(/^\/+/, "");
    const hostPath = rel ? `${workspaceRoot}/${rel}` : workspaceRoot;
    return buildIdeUrl(workspaceRoot, hostPath === workspaceRoot ? undefined : hostPath);
  }

  const folder = workspaceRootFromCwd(path) || workspaceRoot || path;
  return buildIdeUrl(folder, path === folder ? undefined : path);
}

export interface IdeLinkContext {
  chatId?: string;
  cwd?: string;
}

function workspaceRootFromCwd(cwd?: string): string {
  const path = normalizeAbsolutePath(cwd || "");
  if (!path) return "";
  const marker = workspaceMarkerIndex(path);
  if (marker >= 0) return path.slice(0, marker + workspaceSegment.length);
  return path;
}

function isContainerWorkspacePath(path: string): boolean {
  return path === containerWorkspacePath || path.startsWith(`${containerWorkspacePath}/`);
}

function isHostWorkspacePath(path: string): boolean {
  return workspaceMarkerIndex(path) >= 0;
}

function workspaceMarkerIndex(path: string): number {
  if (path === workspaceSegment) return 0;
  if (path.endsWith(workspaceSegment)) return path.length - workspaceSegment.length;
  return path.indexOf(`${workspaceSegment}/`);
}

function normalizeAbsolutePath(path: string): string {
  const trimmed = path.trim();
  if (!trimmed.startsWith("/")) return "";
  return trimmed.replace(/\/{2,}/g, "/").replace(/\/+$/, "") || "/";
}

function stripPathSuffix(href: string): string {
  const hashIndex = href.indexOf("#");
  const queryIndex = href.indexOf("?");
  const cut = [hashIndex, queryIndex].filter((index) => index >= 0).sort((a, b) => a - b)[0];
  return cut === undefined ? href : href.slice(0, cut);
}
