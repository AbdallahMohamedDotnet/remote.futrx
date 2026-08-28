import type {
  ChatEvent,
  InteractionAnswers,
  PromptExecutionPreferences,
} from "../models/chat";

export interface ChatStream {
  readonly isOpen: boolean;
  sendPrompt(
    text: string,
    preferences: PromptExecutionPreferences,
    clientId?: string,
  ): boolean;
  sendInteractionResponse(id: string, answers: InteractionAnswers): boolean;
  sendInteractionActivity(id: string): boolean;
  cancel(): boolean;
  close(): void;
}

export interface ChatStreamCallbacks {
  onOpen: () => void;
  onEvent: (event: ChatEvent) => void;
  onClose: () => void;
}
