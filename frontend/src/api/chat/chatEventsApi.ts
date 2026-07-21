import { requestJson } from "../apiRequest";
import { openChatStream } from "./chatStream";
import type { ChatEventPage } from "../../models/chat";
import type { ChatStream, ChatStreamCallbacks } from "../../types/chatApi";
import { API_ROUTES } from "../../config/routes";

export const chatEventsApi = {
  fetchEvents: (
    id: string,
    params: { limit?: number; before?: number } = {}
  ) => {
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

  openStream: (
    id: string,
    latestSeq: () => number,
    callbacks: ChatStreamCallbacks
  ): ChatStream => openChatStream(id, latestSeq, callbacks),
};
