import { WebSocketConnection } from "../transport/webSocketConnection";
import { webSocketUrl } from "../transport/websocket";

export interface TerminalConnection {
  readonly isOpen: boolean;
  sendInput(data: string): void;
  resize(cols: number, rows: number): void;
  close(): void;
}

interface TerminalConnectionCallbacks {
  onOpen: () => void;
  onOutput: (data: string | Uint8Array) => void;
  onError: () => void;
  onClose: () => void;
}

export const terminalApi = {
  connect(chatId: string, callbacks: TerminalConnectionCallbacks): TerminalConnection {
    return new WebSocketTerminalConnection(chatId, callbacks);
  },
};

class WebSocketTerminalConnection implements TerminalConnection {
  readonly #connection: WebSocketConnection;

  constructor(chatId: string, callbacks: TerminalConnectionCallbacks) {
    this.#connection = new WebSocketConnection({
      url: webSocketUrl(`/ws/terminal?chat=${encodeURIComponent(chatId)}`),
      binaryType: "arraybuffer",
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
    this.#connection.send(JSON.stringify({ type: "input", data }));
  }

  resize(cols: number, rows: number): void {
    if (!this.isOpen) return;
    this.#connection.send(JSON.stringify({ type: "resize", cols, rows }));
  }

  close(): void {
    this.#connection.close();
  }
}
