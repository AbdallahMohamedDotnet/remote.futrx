import { ReconnectingJsonWebSocket } from "../../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../../transport/webSocketUrl";
import type {
  ChatEvent,
  InteractionAnswers,
  PromptExecutionPreferences,
} from "../../models/chat";
import type { ChatStream, ChatStreamCallbacks } from "../../types/chatApi";
import { WEB_SOCKET_ROUTES } from "../../config/routes";
import { CHAT_STREAM_MESSAGE_TYPES } from "../../config/api";
import {
  interactionActivityMessage,
  interactionResponseMessage,
} from "./chatStreamMessages";

export function openChatStream(
  chatId: string,
  latestSeq: () => number,
  callbacks: ChatStreamCallbacks
): ChatStream {
  const stream = new ReconnectingChatStream(chatId, latestSeq, callbacks);
  stream.open();
  return stream;
}

class ReconnectingChatStream implements ChatStream {
  readonly #connection: ReconnectingJsonWebSocket<ChatEvent>;

  constructor(
    chatId: string,
    latestSeq: () => number,
    callbacks: ChatStreamCallbacks
  ) {
    this.#connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl(WEB_SOCKET_ROUTES.chat(chatId, latestSeq())),
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

  sendPrompt(
    text: string,
    preferences: PromptExecutionPreferences,
    clientId?: string,
  ): boolean {
    return this.#connection.send({
      type: CHAT_STREAM_MESSAGE_TYPES.prompt,
      text,
      clientId,
      ...preferences,
    });
  }

  sendInteractionResponse(id: string, answers: InteractionAnswers): boolean {
    return this.#connection.send(interactionResponseMessage(id, answers));
  }

  sendInteractionActivity(id: string): boolean {
    return this.#connection.send(interactionActivityMessage(id));
  }

  cancel(): boolean {
    return this.#connection.send({ type: CHAT_STREAM_MESSAGE_TYPES.cancel });
  }

  close(): void {
    this.#connection.stop();
  }
}
