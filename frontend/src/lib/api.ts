// Thin fetch wrapper. All endpoints are cookie-authenticated by Caddy.
import type { Session, ChatMeta, ChatEvent, ProjectMeta } from "../types";

async function json<T>(method: string, url: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  };
  const r = await fetch(url, init);
  if (r.status === 401) {
    // Session expired or never had one — bounce to a fresh page load so the
    // SPA re-runs useAuth, sees authenticated:false, and shows LoginScreen.
    location.reload();
    // Never resolves — we're navigating away.
    return new Promise<T>(() => {});
  }
  if (!r.ok) {
    let msg = `${r.status}`;
    try { msg = (await r.json()).error || msg; } catch {}
    throw new Error(msg);
  }
  if (r.status === 204) return undefined as T;
  return r.json() as Promise<T>;
}

// --- tmux sessions (kept for legacy / SSH bridge — UI no longer surfaces) --

export const sessionsApi = {
  list: () => json<Session[]>("GET", "/api/sessions"),
  create: (name: string) => json<{ name: string }>("POST", "/api/sessions", { name }),
  kill: (name: string) => json<{ ok: boolean }>("DELETE", `/api/sessions/${encodeURIComponent(name)}`),
  send: (name: string, text: string, pressEnter = true) =>
    json<{ ok: boolean }>("POST", `/api/sessions/${encodeURIComponent(name)}/send`, { text, pressEnter }),
};

// --- chats (claude side) ---------------------------------------------------

export const chatsApi = {
  list: () => json<ChatMeta[]>("GET", "/api/chats"),
  create: (body: {
    tmuxSession?: string;
    cwd?: string;
    title?: string;
    model?: string;
    projectId?: string;
  } = {}) =>
    json<ChatMeta>("POST", "/api/chats", body),
  get: (id: string) => json<ChatMeta>("GET", `/api/chats/${encodeURIComponent(id)}`),
  patch: (id: string, body: { title?: string; cwd?: string; model?: string }) =>
    json<ChatMeta>("PATCH", `/api/chats/${encodeURIComponent(id)}`, body),
  delete: (id: string) =>
    json<{ ok: boolean }>("DELETE", `/api/chats/${encodeURIComponent(id)}`),
  events: (id: string) =>
    json<ChatEvent[]>("GET", `/api/chats/${encodeURIComponent(id)}/events`),
};

// --- projects (LXC containers) --------------------------------------------

export const projectsApi = {
  list: () => json<ProjectMeta[]>("GET", "/api/projects"),
  create: (name: string) => json<ProjectMeta>("POST", "/api/projects", { name }),
  get: (id: string) => json<ProjectMeta>("GET", `/api/projects/${encodeURIComponent(id)}`),
  patch: (id: string, body: { name?: string }) =>
    json<ProjectMeta>("PATCH", `/api/projects/${encodeURIComponent(id)}`, body),
  delete: (id: string) =>
    json<{ ok: boolean }>("DELETE", `/api/projects/${encodeURIComponent(id)}`),
  start: (id: string) =>
    json<ProjectMeta>("POST", `/api/projects/${encodeURIComponent(id)}/start`, {}),
  stop: (id: string) =>
    json<ProjectMeta>("POST", `/api/projects/${encodeURIComponent(id)}/stop`, {}),
};

// --- claude CLI authentication on the server ------------------------------
// Surfaces ~/.claude/.credentials.json status + bridges `claude auth login`
// through HTTP so the admin can authenticate without SSH.

export const claudeAuthApi = {
  status: () => json<{ authenticated: boolean }>("GET", "/api/claude/auth-status"),
  startLogin: () => json<{ url: string; resumed?: boolean }>("POST", "/api/claude/login/start", {}),
  submitCode: (code: string) => json<{ success: boolean }>("POST", "/api/claude/login/code", { code }),
  cancelLogin: () => json<{ ok: boolean }>("POST", "/api/claude/login/cancel", {}),
};

// Multipart upload to chat's working directory. Returns the resolved cwd
// (in case it changed) and per-file results.
export async function uploadChatFiles(
  chatId: string,
  files: File[]
): Promise<{ cwd: string; results: Array<{ name: string; path?: string; size?: number; error?: string }> }> {
  const fd = new FormData();
  for (const f of files) fd.append("files", f, f.name);
  const r = await fetch(`/api/chats/${encodeURIComponent(chatId)}/upload`, {
    method: "POST", body: fd,
  });
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || `upload failed: ${r.status}`);
  return data;
}
