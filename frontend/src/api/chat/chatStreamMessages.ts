import type { ClientToServer, InteractionAnswers } from "../../models/chat";

export function interactionResponseMessage(
  id: string,
  answers: InteractionAnswers,
): ClientToServer {
  return {
    type: "interaction_response",
    id,
    answers,
  };
}

export function interactionActivityMessage(id: string): ClientToServer {
  return { type: "interaction_activity", id };
}
