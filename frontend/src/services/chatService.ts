import { json, request } from "../transport/http";
import type { FileTreeResponse } from "../models/files";
import type { ChatEventPage, ChatMeta, CreateChatInput, UpdateChatInput } from "../models/chat";
import { DirtyWorkingTreeError, type GitHistoryCheckoutResponse, type GitHistoryCommitsResponse, type GitHistoryDiffResponse, type GitHistoryReposResponse } from "../models/history";

export const chatService = {
  list: () => json<ChatMeta[]>("GET", "/api/chats"),
  create: (body: CreateChatInput = {}) => json<ChatMeta>("POST", "/api/chats", body),
  get: (id: string) => json<ChatMeta>("GET", `/api/chats/${encodeURIComponent(id)}`),
  update: (id: string, body: UpdateChatInput) =>
    json<ChatMeta>("PATCH", `/api/chats/${encodeURIComponent(id)}`, body),
  markRead: (id: string) =>
    json<ChatMeta>("POST", `/api/chats/${encodeURIComponent(id)}/read`, {}),
  markUnread: (id: string) =>
    json<ChatMeta>("POST", `/api/chats/${encodeURIComponent(id)}/unread`, {}),
  delete: (id: string) =>
    json<{ ok: boolean }>("DELETE", `/api/chats/${encodeURIComponent(id)}`),
  fork: (id: string) =>
    json<ChatMeta>("POST", `/api/chats/${encodeURIComponent(id)}/fork`, {}),
  files: (id: string) =>
    json<FileTreeResponse>("GET", `/api/chats/${encodeURIComponent(id)}/files`),
  fileDownloadUrl: (id: string, dir: string, path: string) =>
    `/api/chats/${encodeURIComponent(id)}/files/download?dir=${encodeURIComponent(dir)}&path=${encodeURIComponent(path)}`,
  folderDownloadUrl: (id: string, dir: string, path = "") =>
    `/api/chats/${encodeURIComponent(id)}/files/download-folder?dir=${encodeURIComponent(dir)}${path ? `&path=${encodeURIComponent(path)}` : ""}`,
  events: (id: string, params: { limit?: number; before?: number } = {}) => {
    const search = new URLSearchParams();
    if (params.limit) search.set("limit", String(params.limit));
    if (params.before) search.set("before", String(params.before));
    const query = search.toString();
    return json<ChatEventPage>(
      "GET",
      `/api/chats/${encodeURIComponent(id)}/events${query ? `?${query}` : ""}`
    );
  },
  rewind: (id: string, beforeT: number) =>
    json<ChatEventPage>("POST", `/api/chats/${encodeURIComponent(id)}/rewind`, { beforeT }),
  historyRepos: (id: string) =>
    json<GitHistoryReposResponse>("GET", `/api/chats/${encodeURIComponent(id)}/history/repos`),
  historyCommits: (id: string, repo: string, limit = 100) => {
    const search = new URLSearchParams({ repo, limit: String(limit) });
    return json<GitHistoryCommitsResponse>(
      "GET",
      `/api/chats/${encodeURIComponent(id)}/history/commits?${search.toString()}`
    );
  },
  historyDiff: (id: string, repo: string, sha: string) => {
    const search = new URLSearchParams({ repo, sha });
    return json<GitHistoryDiffResponse>(
      "GET",
      `/api/chats/${encodeURIComponent(id)}/history/diff?${search.toString()}`
    );
  },
  historyCheckout: async (id: string, repo: string, sha: string, checkpointMessage = "") => {
    const response = await request("POST", `/api/chats/${encodeURIComponent(id)}/history/checkout`, {
      repo,
      sha,
      checkpointMessage,
    });
    if (response.status === 401) {
      location.reload();
      return new Promise<GitHistoryCheckoutResponse>(() => {});
    }
    if (!response.ok) {
      let body: { error?: string; dirty?: boolean; dirtyFiles?: string[] } = {};
      try {
        body = await response.json();
      } catch {}
      if (response.status === 409 && body.dirty) {
        throw new DirtyWorkingTreeError(body.error || "dirty working tree", body.dirtyFiles || []);
      }
      throw new Error(body.error || String(response.status));
    }
    return response.json() as Promise<GitHistoryCheckoutResponse>;
  },
};
