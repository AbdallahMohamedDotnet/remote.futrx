import type { ChatEvent } from "../models/chat";

export interface ChatStream {
  readonly isOpen: boolean;
  sendPrompt(text: string, clientId?: string): boolean;
  cancel(): boolean;
  close(): void;
}

export interface ChatStreamCallbacks {
  onOpen: () => void;
  onEvent: (event: ChatEvent) => void;
  onClose: () => void;
}
