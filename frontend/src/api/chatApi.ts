import { requestJson } from "./apiRequest";
import { sendHttpRequest } from "../transport/http";
import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/webSocketUrl";
import type { FileTreeResponse } from "../models/files";
import type { ChatEvent, ChatEventPage, ChatMeta, CreateChatInput, UpdateChatInput } from "../models/chat";
import { DirtyWorkingTreeError, type GitHistoryCheckoutResponse, type GitHistoryCommitsResponse, type GitHistoryDiffResponse, type GitHistoryReposResponse } from "../models/history";
import type { ChatStream, ChatStreamCallbacks } from "../types/chatApi";

export const chatApi = {
  list: () => requestJson<ChatMeta[]>("GET", "/api/chats"),
  create: (body: CreateChatInput = {}) => requestJson<ChatMeta>("POST", "/api/chats", body),
  get: (id: string) => requestJson<ChatMeta>("GET", `/api/chats/${encodeURIComponent(id)}`),
  update: (id: string, body: UpdateChatInput) =>
    requestJson<ChatMeta>("PATCH", `/api/chats/${encodeURIComponent(id)}`, body),
  markRead: (id: string) =>
    requestJson<ChatMeta>("POST", `/api/chats/${encodeURIComponent(id)}/read`, {}),
  markUnread: (id: string) =>
    requestJson<ChatMeta>("POST", `/api/chats/${encodeURIComponent(id)}/unread`, {}),
  delete: (id: string) =>
    requestJson<{ ok: boolean }>("DELETE", `/api/chats/${encodeURIComponent(id)}`),
  fork: (id: string) =>
    requestJson<ChatMeta>("POST", `/api/chats/${encodeURIComponent(id)}/fork`, {}),
  files: (id: string) =>
    requestJson<FileTreeResponse>("GET", `/api/chats/${encodeURIComponent(id)}/files`),
  fileDownloadUrl: (id: string, dir: string, path: string) =>
    `/api/chats/${encodeURIComponent(id)}/files/download?dir=${encodeURIComponent(dir)}&path=${encodeURIComponent(path)}`,
  folderDownloadUrl: (id: string, dir: string, path = "") =>
    `/api/chats/${encodeURIComponent(id)}/files/download-folder?dir=${encodeURIComponent(dir)}${path ? `&path=${encodeURIComponent(path)}` : ""}`,
  events: (id: string, params: { limit?: number; before?: number } = {}) => {
    const search = new URLSearchParams();
    if (params.limit) search.set("limit", String(params.limit));
    if (params.before) search.set("before", String(params.before));
    const query = search.toString();
    return requestJson<ChatEventPage>(
      "GET",
      `/api/chats/${encodeURIComponent(id)}/events${query ? `?${query}` : ""}`
    );
  },
  rewind: (id: string, beforeT: number) =>
    requestJson<ChatEventPage>("POST", `/api/chats/${encodeURIComponent(id)}/rewind`, { beforeT }),
  historyRepos: (id: string) =>
    requestJson<GitHistoryReposResponse>("GET", `/api/chats/${encodeURIComponent(id)}/history/repos`),
  historyCommits: (id: string, repo: string, limit = 100) => {
    const search = new URLSearchParams({ repo, limit: String(limit) });
    return requestJson<GitHistoryCommitsResponse>(
      "GET",
      `/api/chats/${encodeURIComponent(id)}/history/commits?${search.toString()}`
    );
  },
  historyDiff: (id: string, repo: string, sha: string) => {
    const search = new URLSearchParams({ repo, sha });
    return requestJson<GitHistoryDiffResponse>(
      "GET",
      `/api/chats/${encodeURIComponent(id)}/history/diff?${search.toString()}`
    );
  },
  historyCheckout: async (id: string, repo: string, sha: string, checkpointMessage = "") => {
    const response = await sendHttpRequest("POST", `/api/chats/${encodeURIComponent(id)}/history/checkout`, {
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
  openStream: (
    id: string,
    latestSeq: () => number,
    callbacks: ChatStreamCallbacks
  ): ChatStream => {
    const stream = new ReconnectingChatStream(id, latestSeq, callbacks);
    stream.open();
    return stream;
  },
};

class ReconnectingChatStream implements ChatStream {
  readonly #connection: ReconnectingJsonWebSocket<ChatEvent>;

  constructor(
    chatId: string,
    latestSeq: () => number,
    callbacks: ChatStreamCallbacks
  ) {
    this.#connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => chatStreamUrl(chatId, latestSeq()),
      onOpen: callbacks.onOpen,
      onMessage: callbacks.onEvent,
      onClose: callbacks.onClose,
    });
  }

  get isOpen(): boolean {
    return this.#connection.isOpen;
  }

  open(): void {
    this.#connection.start();
  }

  sendPrompt(text: string): boolean {
    return this.#connection.send({ type: "prompt", text });
  }

  cancel(): boolean {
    return this.#connection.send({ type: "cancel" });
  }

  close(): void {
    this.#connection.stop();
  }
}

function chatStreamUrl(chatId: string, sinceSeq: number): string {
  const url = webSocketUrl(`/ws/chat/${encodeURIComponent(chatId)}`);
  return sinceSeq > 0 ? `${url}?since=${sinceSeq}` : url;
}
