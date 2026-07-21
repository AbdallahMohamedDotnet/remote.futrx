import { WebSocketConnection } from "../transport/webSocketConnection";
import { webSocketUrl } from "../transport/webSocketUrl";
import type {
  TerminalConnection,
  TerminalConnectionCallbacks,
} from "../types/terminal";
import { WEB_SOCKET_ROUTES } from "../config/routes";
import { TERMINAL_WEB_SOCKET_BINARY_TYPE } from "../config/terminal";
import { TERMINAL_MESSAGE_TYPES } from "../config/protocol";

export const terminalApi = {
  connect(chatId: string, callbacks: TerminalConnectionCallbacks): TerminalConnection {
    return new WebSocketTerminalConnection(chatId, callbacks);
  },
};

class WebSocketTerminalConnection implements TerminalConnection {
  readonly #connection: WebSocketConnection;

  constructor(chatId: string, callbacks: TerminalConnectionCallbacks) {
    this.#connection = new WebSocketConnection({
      url: webSocketUrl(WEB_SOCKET_ROUTES.terminal(chatId)),
      binaryType: TERMINAL_WEB_SOCKET_BINARY_TYPE,
      onOpen: callbacks.onOpen,
      onMessage(data) {
        callbacks.onOutput(
          data instanceof ArrayBuffer ? new Uint8Array(data) : String(data)
        );
      },
      onError: callbacks.onError,
      onClose: callbacks.onClose,
    });
  }

  get isOpen(): boolean {
    return this.#connection.isOpen;
  }

  sendInput(data: string): void {
    if (!this.isOpen) return;
    this.#connection.send(
      JSON.stringify({ type: TERMINAL_MESSAGE_TYPES.input, data })
    );
  }

  resize(cols: number, rows: number): void {
    if (!this.isOpen) return;
    this.#connection.send(
      JSON.stringify({ type: TERMINAL_MESSAGE_TYPES.resize, cols, rows })
    );
  }

  close(): void {
    this.#connection.close();
  }
}
