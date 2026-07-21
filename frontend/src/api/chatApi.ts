import { requestJson } from "./apiRequest";
import { chatEventsApi } from "./chat/chatEventsApi";
import { chatFilesApi } from "./chat/chatFilesApi";
import { chatHistoryApi } from "./chat/chatHistoryApi";
import type { ChatMeta, CreateChatInput, UpdateChatInput } from "../models/chat";
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
  events: chatEventsApi.events,
  rewind: chatEventsApi.rewind,
  historyRepos: chatHistoryApi.historyRepos,
  historyCommits: chatHistoryApi.historyCommits,
  historyDiff: chatHistoryApi.historyDiff,
  historyCheckout: chatHistoryApi.historyCheckout,
  openStream: chatEventsApi.openStream,
};
