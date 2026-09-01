import {
  askUserQuestionState,
  type QuestionChoiceState,
} from "../../../../state/hooks/chat/askUserQuestionState.ts";

export type { QuestionChoiceState } from "../../../../state/hooks/chat/askUserQuestionState.ts";

export function createQuestionChoiceState(count: number): Record<number, QuestionChoiceState> {
  return askUserQuestionState.createChoices(count);
}

export function toggleQuestionOption(
  current: QuestionChoiceState,
  optionIndex: number,
  multi: boolean,
  allowOptionNotes: boolean,
): QuestionChoiceState {
  return askUserQuestionState.toggleOption(current, optionIndex, multi, allowOptionNotes);
}

export function activateQuestionFreeform(
  current: QuestionChoiceState,
  multi: boolean,
  allowOptionNotes: boolean,
): QuestionChoiceState {
  return askUserQuestionState.activateFreeform(current, multi, allowOptionNotes);
}
