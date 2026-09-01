import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type {
  AskUserQuestionInput,
  QuestionAnswerSubmission,
} from "../../../models/chat.ts";
import { chatQuestionService } from "../../../services/chat/chatQuestionService.ts";
import { chatQuestionStorageService } from "../../../services/chat/chatQuestionStorageService.ts";
import { askUserQuestionState } from "./askUserQuestionState.ts";

export function useAskUserQuestion({
  chatId,
  toolUseId,
  input,
  onSubmit,
  awaitResolution = false,
  allowOptionNotes = false,
  onActivity,
}: {
  chatId: string;
  toolUseId: string;
  input: AskUserQuestionInput;
  onSubmit: (answer: QuestionAnswerSubmission) => boolean;
  awaitResolution?: boolean;
  allowOptionNotes?: boolean;
  onActivity?: () => boolean;
}) {
  const questions = input.questions ?? [];
  const total = questions.length;
  const sensitive = questions.some((question) => question.isSecret);
  const initialAnswered = useMemo(
    () => sensitive || awaitResolution
      ? null
      : chatQuestionStorageService.readAnswered(chatId, toolUseId),
    [awaitResolution, chatId, sensitive, toolUseId],
  );
  const [answered, setAnswered] = useState<string | null>(initialAnswered);
  const [page, setPage] = useState(0);
  const [choices, setChoices] = useState(() =>
    askUserQuestionState.createChoices(total)
  );
  const [other, setOther] = useState<Record<number, string>>({});
  const [autoResolutionSnoozed, setAutoResolutionSnoozed] = useState(false);
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const activityReported = useRef(false);

  useEffect(() => {
    setAnswered(initialAnswered);
  }, [initialAnswered]);

  useEffect(() => {
    if (sensitive) chatQuestionStorageService.clearAnswered(chatId, toolUseId);
  }, [chatId, sensitive, toolUseId]);

  useEffect(() => {
    activityReported.current = false;
    setPage(0);
    setChoices(askUserQuestionState.createChoices(total));
    setOther({});
    setAutoResolutionSnoozed(false);
    setSubmissionError(null);
  }, [toolUseId, total]);

  function reportActivity() {
    if (activityReported.current || !onActivity) return;
    if (onActivity()) {
      activityReported.current = true;
      setAutoResolutionSnoozed(true);
    }
  }

  function toggle(qi: number, oi: number, multi: boolean) {
    reportActivity();
    setChoices((previous) => ({
      ...previous,
      [qi]: askUserQuestionState.toggleOption(
        previous[qi] ?? { selected: new Set(), freeformActive: false },
        oi,
        multi,
        allowOptionNotes,
      ),
    }));
  }

  function activateOther(qi: number, multi: boolean) {
    reportActivity();
    setChoices((previous) => ({
      ...previous,
      [qi]: askUserQuestionState.activateFreeform(
        previous[qi] ?? { selected: new Set(), freeformActive: false },
        multi,
        allowOptionNotes,
      ),
    }));
  }

  function setOtherText(qi: number, value: string) {
    reportActivity();
    setOther((prev) => ({ ...prev, [qi]: value }));
  }

  function questionAnswered(qi: number): boolean {
    const choice = choices[qi] ?? { selected: new Set<number>(), freeformActive: false };
    const otherText = choice.freeformActive ? (other[qi] || "").trim() : "";
    if (otherText.length > 0) return true;
    return choice.selected.size > 0;
  }

  function chosenLabels(qi: number): string[] {
    const question = questions[qi];
    if (!question) return [];
    const chosen: string[] = [];
    const choice = choices[qi] ?? { selected: new Set<number>(), freeformActive: false };
    choice.selected.forEach((optionIndex) => {
      const option = question.options?.[optionIndex];
      if (option) chosen.push(option.label);
    });
    if (choice.freeformActive) {
      const text = (other[qi] || "").trim();
      if (text) chosen.push(text);
    }
    return chosen;
  }

  function summarize(): QuestionAnswerSubmission {
    return chatQuestionService.summarizeAnswers(questions, chosenLabels);
  }

  function submit() {
    const summary = summarize();
    if (!onSubmit(summary)) {
      setSubmissionError(
        "The agent is no longer waiting for this response. Restart the turn and try again.",
      );
      return;
    }
    setSubmissionError(null);
    if (sensitive) {
      // Clear Remote-owned UI state once the provider channel accepts the
      // response. Codex still receives the value and owns its session history.
      setChoices(askUserQuestionState.createChoices(total));
      setOther({});
    }
    if (awaitResolution) return;
    if (sensitive) {
      setAnswered("Secret response received");
      return;
    }
    chatQuestionStorageService.writeAnswered(chatId, toolUseId, summary.preview);
    setAnswered(summary.preview);
  }

  return {
    questions,
    total,
    answered,
    page,
    setPage,
    currentQuestion: questions[page],
    selectedOptions: choices[page]?.selected ?? new Set<number>(),
    isOtherActive: choices[page]?.freeformActive ?? false,
    otherText: other[page] || "",
    autoResolutionSnoozed,
    submissionError,
    canAdvance: questionAnswered(page),
    questionAnswered,
    reportActivity,
    toggle,
    activateOther,
    setOtherText,
    submit,
  };
}
