const INITIAL_RECONNECT_DELAY_MS = 400;
const MAX_RECONNECT_DELAY_MS = 5_000;

interface ReconnectingJsonWebSocketOptions<TMessage> {
  resolveUrl: () => string;
  onMessage: (message: TMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
}

export class ReconnectingJsonWebSocket<TMessage> {
  readonly #configuration: ReconnectingJsonWebSocketOptions<TMessage>;
  #socket: WebSocket | null = null;
  #reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  #reconnectAttempt = 0;
  #isStopped = true;

  constructor(options: ReconnectingJsonWebSocketOptions<TMessage>) {
    this.#configuration = options;
  }

  get isOpen(): boolean {
    return this.#socket?.readyState === WebSocket.OPEN;
  }

  start(): void {
    if (!this.#isStopped) return;
    this.#isStopped = false;
    this.#connect();
  }

  stop(): void {
    this.#isStopped = true;
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
    if (this.#isStopped) return;
    const delayMs = Math.min(
      MAX_RECONNECT_DELAY_MS,
      INITIAL_RECONNECT_DELAY_MS * 2 ** this.#reconnectAttempt
    );
    this.#reconnectAttempt++;
    this.#reconnectTimer = setTimeout(() => this.#connect(), delayMs);
  }

  #connect(): void {
    if (this.#isStopped) return;
    const socket = new WebSocket(this.#configuration.resolveUrl());
    this.#socket = socket;

    socket.onopen = () => {
      if (this.#isStopped || this.#socket !== socket) return;
      this.#reconnectAttempt = 0;
      this.#configuration.onOpen?.();
    };

    socket.onmessage = (event) => {
      if (this.#isStopped || this.#socket !== socket) return;
      try {
        this.#configuration.onMessage(JSON.parse(event.data) as TMessage);
      } catch {}
    };

    socket.onclose = () => {
      if (this.#socket !== socket) return;
      this.#socket = null;
      this.#configuration.onClose?.();
      this.#scheduleReconnect();
    };

    socket.onerror = () => {
      try {
        socket.close();
      } catch {}
    };
  }
}
