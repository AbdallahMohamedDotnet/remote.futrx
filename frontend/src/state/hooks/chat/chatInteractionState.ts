import type {
  ChatStatus,
  InteractionAnswers,
  QuestionAnswerRequest,
} from "../../../models/chat";

export interface ChatInteractionTransportState {
  status: ChatStatus;
  wsReady: boolean;
  synced: boolean;
  streamOpen: boolean;
}

export function canSendInteractionResponse({
  status,
  wsReady,
  synced,
  streamOpen,
}: ChatInteractionTransportState): boolean {
  return status === "streaming" && wsReady && synced && streamOpen;
}

export interface QuestionAnswerTransport {
  sendPrompt: (text: string) => boolean;
  sendInteractionResponse: (id: string, answers: InteractionAnswers) => boolean;
}

export function dispatchQuestionAnswer(
  answer: QuestionAnswerRequest,
  transport: QuestionAnswerTransport,
): boolean {
  if (answer.interactionId) {
    return transport.sendInteractionResponse(answer.interactionId, answer.answers);
  }
  // A legacy tool card can only continue by creating an ordinary persisted
  // user prompt. Never put a value presented as secret onto that path.
  if (answer.sensitive) return false;
  return transport.sendPrompt(answer.text);
}
