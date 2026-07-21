export type TerminalStatus = "connecting" | "connected" | "closed" | "error";

export interface TerminalConnection {
  readonly isOpen: boolean;
  sendInput(data: string): void;
  resize(cols: number, rows: number): void;
  close(): void;
}

export interface TerminalConnectionCallbacks {
  onOpen: () => void;
  onOutput: (data: string | Uint8Array) => void;
  onError: () => void;
  onClose: () => void;
}
