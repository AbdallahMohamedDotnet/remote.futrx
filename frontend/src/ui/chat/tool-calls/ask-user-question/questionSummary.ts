import type { Question, QuestionSummary } from "./types";
import { chatQuestionService } from "../../../../services/chat/chatQuestionService.ts";

export function hasValidQuestionIds(questions: Question[]): boolean {
  return chatQuestionService.hasValidIds(questions);
}

export function summarizeQuestionAnswers(
  questions: Question[],
  answersForQuestion: (index: number) => string[],
): QuestionSummary {
  return chatQuestionService.summarizeAnswers(questions, answersForQuestion);
}

export function resolvedQuestionPreview(
  questions: Question[],
  output?: string,
): string {
  return chatQuestionService.resolvedPreview(questions, output);
}
