import { json } from "../api/http";
import type { ChatEvent, ChatMeta, CreateChatInput, UpdateChatInput } from "../models/chat";

export const chatService = {
  list: () => json<ChatMeta[]>("GET", "/api/chats"),
  create: (body: CreateChatInput = {}) => json<ChatMeta>("POST", "/api/chats", body),
  get: (id: string) => json<ChatMeta>("GET", `/api/chats/${encodeURIComponent(id)}`),
  update: (id: string, body: UpdateChatInput) =>
    json<ChatMeta>("PATCH", `/api/chats/${encodeURIComponent(id)}`, body),
  delete: (id: string) =>
    json<{ ok: boolean }>("DELETE", `/api/chats/${encodeURIComponent(id)}`),
  events: (id: string) =>
    json<ChatEvent[]>("GET", `/api/chats/${encodeURIComponent(id)}/events`),
  rewind: (id: string, beforeT: number) =>
    json<{ events: ChatEvent[] }>("POST", `/api/chats/${encodeURIComponent(id)}/rewind`, { beforeT }),
};
