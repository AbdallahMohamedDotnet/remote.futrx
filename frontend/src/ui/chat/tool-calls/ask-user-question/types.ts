import type { QuestionAnswerSubmission } from "../../../../models/chat";

export interface Question {
  id?: string;
  question: string;
  header?: string;
  multiSelect?: boolean;
  isOther?: boolean;
  isSecret?: boolean;
  options?: Array<{ label: string; description?: string }> | null;
}

export interface AskUserQuestionInput {
  questions?: Question[];
  isBlocking?: boolean;
  autoResolutionMs?: number | null;
}

export type QuestionSummary = QuestionAnswerSubmission;
