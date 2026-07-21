export interface ReconnectingJsonWebSocketOptions<TMessage> {
  resolveUrl: () => string;
  onMessage: (message: TMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
}

export class ReconnectingJsonWebSocket<TMessage> {
  readonly #options: ReconnectingJsonWebSocketOptions<TMessage>;
  #socket: WebSocket | null = null;
  #reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  #attempt = 0;
  #stopped = true;

  constructor(options: ReconnectingJsonWebSocketOptions<TMessage>) {
    this.#options = options;
  }

  get isOpen(): boolean {
    return this.#socket?.readyState === WebSocket.OPEN;
  }

  start(): void {
    if (!this.#stopped) return;
    this.#stopped = false;
    this.#connect();
  }

  stop(): void {
    this.#stopped = true;
    if (this.#reconnectTimer !== null) {
      clearTimeout(this.#reconnectTimer);
      this.#reconnectTimer = null;
    }
    const socket = this.#socket;
    this.#socket = null;
    try {
      socket?.close();
    } catch {}
  }

  send(message: unknown): boolean {
    if (!this.isOpen || this.#socket === null) return false;
    this.#socket.send(JSON.stringify(message));
    return true;
  }

  #scheduleReconnect(): void {
    if (this.#stopped) return;
    const delay = Math.min(5000, 400 * 2 ** this.#attempt);
    this.#attempt++;
    this.#reconnectTimer = setTimeout(() => this.#connect(), delay);
  }

  #connect(): void {
    if (this.#stopped) return;
    const socket = new WebSocket(this.#options.resolveUrl());
    this.#socket = socket;

    socket.onopen = () => {
      if (this.#stopped || this.#socket !== socket) return;
      this.#attempt = 0;
      this.#options.onOpen?.();
    };

    socket.onmessage = (event) => {
      if (this.#stopped || this.#socket !== socket) return;
      try {
        this.#options.onMessage(JSON.parse(event.data) as TMessage);
      } catch {}
    };

    socket.onclose = () => {
      if (this.#socket !== socket) return;
      this.#socket = null;
      this.#options.onClose?.();
      this.#scheduleReconnect();
    };

    socket.onerror = () => {
      try {
        socket.close();
      } catch {}
    };
  }
}
