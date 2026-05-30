import { json } from "../api/http";
import type { ChatEventPage, ChatMeta, CreateChatInput, UpdateChatInput } from "../models/chat";

export const chatService = {
  list: () => json<ChatMeta[]>("GET", "/api/chats"),
  create: (body: CreateChatInput = {}) => json<ChatMeta>("POST", "/api/chats", body),
  get: (id: string) => json<ChatMeta>("GET", `/api/chats/${encodeURIComponent(id)}`),
  update: (id: string, body: UpdateChatInput) =>
    json<ChatMeta>("PATCH", `/api/chats/${encodeURIComponent(id)}`, body),
  delete: (id: string) =>
    json<{ ok: boolean }>("DELETE", `/api/chats/${encodeURIComponent(id)}`),
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
};
