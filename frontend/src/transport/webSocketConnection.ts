import type { WebSocketConnectionOptions } from "../types/transport";

export class WebSocketConnection {
  readonly #socket: WebSocket;

  constructor(options: WebSocketConnectionOptions) {
    const socket = new WebSocket(options.url);
    this.#socket = socket;

    if (options.binaryType !== undefined) {
      socket.binaryType = options.binaryType;
    }
    socket.onopen = options.onOpen;
    socket.onmessage = (event) => options.onMessage(event.data);
    socket.onerror = options.onError;
    socket.onclose = options.onClose;
  }

  get isOpen(): boolean {
    return this.#socket.readyState === WebSocket.OPEN;
  }

  send(message: string): void {
    this.#socket.send(message);
  }

  close(): void {
    try {
      this.#socket.close();
    } catch {}
  }
}
