import type { ApplicationPath } from "../types/transport";

function applicationPath(path: ApplicationPath): ApplicationPath {
  return path;
}

export const API_ROUTES = {
  authSession: "/auth/me",
  chats: {
    collection: "/api/chats",
    item: (id: string) => `/api/chats/${encodeURIComponent(id)}`,
    read: (id: string) => `/api/chats/${encodeURIComponent(id)}/read`,
    unread: (id: string) => `/api/chats/${encodeURIComponent(id)}/unread`,
    fork: (id: string) => `/api/chats/${encodeURIComponent(id)}/fork`,
    files: (id: string) => `/api/chats/${encodeURIComponent(id)}/files`,
    fileDownload: (id: string, dir: string, path: string) =>
      `/api/chats/${encodeURIComponent(id)}/files/download?dir=${encodeURIComponent(dir)}&path=${encodeURIComponent(path)}`,
    folderDownload: (id: string, dir: string, path: string) =>
      `/api/chats/${encodeURIComponent(id)}/files/download-folder?dir=${encodeURIComponent(dir)}${path ? `&path=${encodeURIComponent(path)}` : ""}`,
    events: (id: string, query: string) =>
      `/api/chats/${encodeURIComponent(id)}/events${query ? `?${query}` : ""}`,
    rewind: (id: string) => `/api/chats/${encodeURIComponent(id)}/rewind`,
    historyRepos: (id: string) =>
      `/api/chats/${encodeURIComponent(id)}/history/repos`,
    historyCommits: (id: string, query: string) =>
      `/api/chats/${encodeURIComponent(id)}/history/commits?${query}`,
    historyDiff: (id: string, query: string) =>
      `/api/chats/${encodeURIComponent(id)}/history/diff?${query}`,
    historyCheckout: (id: string) =>
      `/api/chats/${encodeURIComponent(id)}/history/checkout`,
  },
  claudeAuth: {
    status: "/api/claude/auth-status",
    startLogin: "/api/claude/login/start",
    submitCode: "/api/claude/login/code",
    cancelLogin: "/api/claude/login/cancel",
  },
  codexAuth: {
    status: "/api/codex/auth-status",
    startDeviceLogin: "/api/codex/login/device",
  },
  kimiAuth: {
    status: "/api/kimi/auth-status",
    startDeviceLogin: "/api/kimi/login/device",
  },
  projects: {
    collection: "/api/projects",
    item: (id: string) => `/api/projects/${encodeURIComponent(id)}`,
    reorder: "/api/projects/reorder",
    start: (id: string) => `/api/projects/${encodeURIComponent(id)}/start`,
    stop: (id: string) => `/api/projects/${encodeURIComponent(id)}/stop`,
    container: (id: string) =>
      `/api/projects/${encodeURIComponent(id)}/container`,
    repairNetwork: (id: string) =>
      `/api/projects/${encodeURIComponent(id)}/repair-network`,
    apps: (id: string) => `/api/projects/${encodeURIComponent(id)}/apps`,
    agentBrowser: (id: string, scope?: string) =>
      `/api/projects/${encodeURIComponent(id)}/agent-browser${scope ? `?scope=${encodeURIComponent(scope)}` : ""}`,
    startAgentBrowser: (id: string) =>
      `/api/projects/${encodeURIComponent(id)}/agent-browser/start`,
    secrets: (id: string) =>
      `/api/projects/${encodeURIComponent(id)}/secrets`,
    secret: (id: string, key: string) =>
      `/api/projects/${encodeURIComponent(id)}/secrets/${encodeURIComponent(key)}`,
    access: (id: string) => `/api/projects/${encodeURIComponent(id)}/access`,
    accessMember: (id: string, email: string) =>
      `/api/projects/${encodeURIComponent(id)}/access/${encodeURIComponent(email)}`,
  },
  settings: "/api/me/settings",
  skills: (query: string) => `/api/skills?${query}`,
  uploads: "/api/uploads",
  users: {
    collection: "/api/admin/users",
    item: (email: string) =>
      `/api/admin/users/${encodeURIComponent(email)}`,
    role: (email: string) =>
      `/api/admin/users/${encodeURIComponent(email)}/role`,
  },
} as const;

export const WEB_SOCKET_ROUTES = {
  workspace: applicationPath("/ws/workspace"),
  claudeAuthStatus: applicationPath("/ws/claude/auth-status"),
  codexAuthStatus: applicationPath("/ws/codex/auth-status"),
  kimiAuthStatus: applicationPath("/ws/kimi/auth-status"),
  chat: (chatId: string, sinceSeq: number): ApplicationPath => {
    const route = applicationPath(`/ws/chat/${encodeURIComponent(chatId)}`);
    return sinceSeq > 0
      ? applicationPath(`${route}?since=${sinceSeq}`)
      : route;
  },
  terminal: (chatId: string): ApplicationPath =>
    applicationPath(`/ws/terminal?chat=${encodeURIComponent(chatId)}`),
} as const;
