import type {
  AnswerQuestionHandler,
  InteractionActivityHandler,
} from "../../../models/chat";

export interface ToolCallProps {
  toolUseId?: string;
  chatId?: string;
  name: string;
  input: Record<string, unknown> | undefined;
  output?: string;
  isError?: boolean;
  status: "running" | "done";
  interactive?: boolean;
  interactionRequestedAt?: number;
  onAnswerQuestion?: AnswerQuestionHandler;
  onInteractionActivity?: InteractionActivityHandler;
}
