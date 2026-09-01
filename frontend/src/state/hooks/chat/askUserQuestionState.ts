export interface QuestionChoiceState {
  selected: Set<number>;
  freeformActive: boolean;
}

class AskUserQuestionState {
  createChoices(count: number): Record<number, QuestionChoiceState> {
    const result: Record<number, QuestionChoiceState> = {};
    for (let index = 0; index < count; index++) {
      result[index] = { selected: new Set(), freeformActive: false };
    }
    return result;
  }

  toggleOption(
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

  activateFreeform(
    current: QuestionChoiceState,
    multi: boolean,
    allowOptionNotes: boolean,
  ): QuestionChoiceState {
    return {
      selected: !multi && !allowOptionNotes ? new Set() : new Set(current.selected),
      freeformActive: true,
    };
  }
}

export const askUserQuestionState = new AskUserQuestionState();
