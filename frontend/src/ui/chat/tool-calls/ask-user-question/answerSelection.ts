export interface QuestionChoiceState {
  selected: Set<number>;
  freeformActive: boolean;
}

export function createQuestionChoiceState(count: number): Record<number, QuestionChoiceState> {
  const result: Record<number, QuestionChoiceState> = {};
  for (let index = 0; index < count; index++) {
    result[index] = { selected: new Set(), freeformActive: false };
  }
  return result;
}

export function toggleQuestionOption(
  current: QuestionChoiceState,
  optionIndex: number,
  multi: boolean,
  allowOptionNotes: boolean,
): QuestionChoiceState {
  const selected = new Set(current.selected);
  if (multi) {
    if (selected.has(optionIndex)) selected.delete(optionIndex);
    else selected.add(optionIndex);
  } else {
    const deselect = allowOptionNotes && selected.size === 1 && selected.has(optionIndex);
    selected.clear();
    if (!deselect) selected.add(optionIndex);
  }
  return {
    selected,
    freeformActive: allowOptionNotes ? current.freeformActive : false,
  };
}

export function activateQuestionFreeform(
  current: QuestionChoiceState,
  multi: boolean,
  allowOptionNotes: boolean,
): QuestionChoiceState {
  return {
    selected: !multi && !allowOptionNotes ? new Set() : new Set(current.selected),
    freeformActive: true,
  };
}
