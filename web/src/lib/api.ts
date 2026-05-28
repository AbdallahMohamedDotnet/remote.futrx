// Thin fetch wrapper. All endpoints are cookie-authenticated by Caddy.
import type { Session, ChatMeta, ChatEvent } from "../types";

async function json<T>(method: string, url: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  };
  const r = await fetch(url, init);
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
  create: (body: { tmuxSession?: string; cwd?: string; title?: string; model?: string } = {}) =>
    json<ChatMeta>("POST", "/api/chats", body),
  get: (id: string) => json<ChatMeta>("GET", `/api/chats/${encodeURIComponent(id)}`),
  patch: (id: string, body: { title?: string; cwd?: string; model?: string }) =>
    json<ChatMeta>("PATCH", `/api/chats/${encodeURIComponent(id)}`, body),
  delete: (id: string) =>
    json<{ ok: boolean }>("DELETE", `/api/chats/${encodeURIComponent(id)}`),
  events: (id: string) =>
    json<ChatEvent[]>("GET", `/api/chats/${encodeURIComponent(id)}/events`),
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
