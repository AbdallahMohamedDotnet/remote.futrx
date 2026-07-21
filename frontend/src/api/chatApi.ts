import { requestJson } from "./apiRequest";
import { openChatStream } from "./chat/chatStream";
import { sendHttpRequest } from "../transport/http";
import type { FileTreeResponse } from "../models/files";
import type { ChatEventPage, ChatMeta, CreateChatInput, UpdateChatInput } from "../models/chat";
import { DirtyWorkingTreeError, type GitHistoryCheckoutResponse, type GitHistoryCommitsResponse, type GitHistoryDiffResponse, type GitHistoryReposResponse } from "../models/history";
import type { ChatStream, ChatStreamCallbacks } from "../types/chatApi";
import { API_ROUTES } from "../config/routes";
import {
  API_RESPONSE_STATUS,
  DEFAULT_CHAT_HISTORY_COMMIT_LIMIT,
  DIRTY_WORKING_TREE_FALLBACK_MESSAGE,
} from "../config/api";

export const chatApi = {
  list: () => requestJson<ChatMeta[]>("GET", API_ROUTES.chats.collection),
  create: (body: CreateChatInput = {}) =>
    requestJson<ChatMeta>("POST", API_ROUTES.chats.collection, body),
  get: (id: string) => requestJson<ChatMeta>("GET", API_ROUTES.chats.item(id)),
  update: (id: string, body: UpdateChatInput) =>
    requestJson<ChatMeta>("PATCH", API_ROUTES.chats.item(id), body),
  markRead: (id: string) =>
    requestJson<ChatMeta>("POST", API_ROUTES.chats.read(id), {}),
  markUnread: (id: string) =>
    requestJson<ChatMeta>("POST", API_ROUTES.chats.unread(id), {}),
  delete: (id: string) =>
    requestJson<{ ok: boolean }>("DELETE", API_ROUTES.chats.item(id)),
  fork: (id: string) =>
    requestJson<ChatMeta>("POST", API_ROUTES.chats.fork(id), {}),
  files: (id: string) =>
    requestJson<FileTreeResponse>("GET", API_ROUTES.chats.files(id)),
  fileDownloadUrl: (id: string, dir: string, path: string) =>
    API_ROUTES.chats.fileDownload(id, dir, path),
  folderDownloadUrl: (id: string, dir: string, path = "") =>
    API_ROUTES.chats.folderDownload(id, dir, path),
  events: (id: string, params: { limit?: number; before?: number } = {}) => {
    const search = new URLSearchParams();
    if (params.limit) search.set("limit", String(params.limit));
    if (params.before) search.set("before", String(params.before));
    const query = search.toString();
    return requestJson<ChatEventPage>(
      "GET",
      API_ROUTES.chats.events(id, query)
    );
  },
  rewind: (id: string, beforeT: number) =>
    requestJson<ChatEventPage>("POST", API_ROUTES.chats.rewind(id), { beforeT }),
  historyRepos: (id: string) =>
    requestJson<GitHistoryReposResponse>("GET", API_ROUTES.chats.historyRepos(id)),
  historyCommits: (
    id: string,
    repo: string,
    limit = DEFAULT_CHAT_HISTORY_COMMIT_LIMIT
  ) => {
    const search = new URLSearchParams({ repo, limit: String(limit) });
    return requestJson<GitHistoryCommitsResponse>(
      "GET",
      API_ROUTES.chats.historyCommits(id, search.toString())
    );
  },
  historyDiff: (id: string, repo: string, sha: string) => {
    const search = new URLSearchParams({ repo, sha });
    return requestJson<GitHistoryDiffResponse>(
      "GET",
      API_ROUTES.chats.historyDiff(id, search.toString())
    );
  },
  historyCheckout: async (id: string, repo: string, sha: string, checkpointMessage = "") => {
    const response = await sendHttpRequest("POST", API_ROUTES.chats.historyCheckout(id), {
      repo,
      sha,
      checkpointMessage,
    });
    if (response.status === API_RESPONSE_STATUS.unauthorized) {
      location.reload();
      return new Promise<GitHistoryCheckoutResponse>(() => {});
    }
    if (!response.ok) {
      let body: { error?: string; dirty?: boolean; dirtyFiles?: string[] } = {};
      try {
        body = await response.json();
      } catch {}
      if (response.status === API_RESPONSE_STATUS.conflict && body.dirty) {
        throw new DirtyWorkingTreeError(
          body.error || DIRTY_WORKING_TREE_FALLBACK_MESSAGE,
          body.dirtyFiles || []
        );
      }
      throw new Error(body.error || String(response.status));
    }
    return response.json() as Promise<GitHistoryCheckoutResponse>;
  },
  openStream: (
    id: string,
    latestSeq: () => number,
    callbacks: ChatStreamCallbacks
  ): ChatStream => openChatStream(id, latestSeq, callbacks),
};
