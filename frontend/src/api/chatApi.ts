import { requestJson } from "./apiRequest";
import { chatFilesApi } from "./chat/chatFilesApi";
import { chatHistoryApi } from "./chat/chatHistoryApi";
import { openChatStream } from "./chat/chatStream";
import type { ChatEventPage, ChatMeta, CreateChatInput, UpdateChatInput } from "../models/chat";
import type { ChatStream, ChatStreamCallbacks } from "../types/chatApi";
import { API_ROUTES } from "../config/routes";

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
  files: chatFilesApi.files,
  fileDownloadUrl: chatFilesApi.fileDownloadUrl,
  folderDownloadUrl: chatFilesApi.folderDownloadUrl,
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
  historyRepos: chatHistoryApi.historyRepos,
  historyCommits: chatHistoryApi.historyCommits,
  historyDiff: chatHistoryApi.historyDiff,
  historyCheckout: chatHistoryApi.historyCheckout,
  openStream: (
    id: string,
    latestSeq: () => number,
    callbacks: ChatStreamCallbacks
  ): ChatStream => openChatStream(id, latestSeq, callbacks),
};
