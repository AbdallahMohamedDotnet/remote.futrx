import { useEffect, useMemo, useState } from "preact/hooks";
import type { ChatProvider } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import { slashCommandMenuPolicy } from "./slashCommandMenuPolicy";
import { useAvailableSkills } from "./useAvailableSkills";

const NO_SKILLS: RegisteredSkill[] = [];

export interface SlashCommandMenuState {
  open: boolean;
  loading: boolean;
  error: string;
  query: string;
  items: RegisteredSkill[];
  highlight: number;
  setHighlight: (index: number) => void;
  choose: (skill: RegisteredSkill) => void;
  onKeyDown: (event: KeyboardEvent) => boolean;
}

export function useSlashCommandMenu({
  provider,
  projectId,
  text,
  onSelectSkill,
  onTextChange,
  focusTextarea,
}: {
  provider: ChatProvider;
  projectId?: string;
  text: string;
  onSelectSkill: (skill: RegisteredSkill) => void;
  onTextChange: (text: string) => void;
  focusTextarea: () => void;
}): SlashCommandMenuState {
  const { skills, loading, error } = useAvailableSkills(provider, projectId);
  const [highlight, setHighlight] = useState(0);
  const [dismissed, setDismissed] = useState(false);
  const menu = useMemo(
    () => slashCommandMenuPolicy.resolve(text, skills),
    [text, skills],
  );
  const triggered = menu !== null;
  const query = menu?.query ?? null;
  const items = menu?.items ?? NO_SKILLS;

  // Escape only hides the palette for the current token; once the trigger goes
  // away (text cleared or a space typed) a fresh "/" should open it again.
  useEffect(() => {
    if (!triggered) setDismissed(false);
  }, [triggered]);

  // Keep the highlight on the first row whenever the visible list changes.
  useEffect(() => {
    setHighlight(0);
  }, [query, skills]);

  const open = triggered && !dismissed;
  const safeHighlight = slashCommandMenuPolicy.clampHighlight(highlight, items.length);

  function choose(skill: RegisteredSkill) {
    onSelectSkill(skill);
    onTextChange("");
    setDismissed(false);
    focusTextarea();
  }

  function onKeyDown(event: KeyboardEvent): boolean {
    if (!open) return false;
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        setHighlight((index) => slashCommandMenuPolicy.moveHighlight(index, 1, items.length));
        return true;
      case "ArrowUp":
        event.preventDefault();
        setHighlight((index) => slashCommandMenuPolicy.moveHighlight(index, -1, items.length));
        return true;
      case "Tab":
      case "Enter": {
        // Leave newline (Shift+Enter) and the send shortcut (Ctrl/Cmd+Enter)
        // to the textarea; only plain Enter/Tab confirms a command.
        if (event.key === "Enter" && (event.shiftKey || event.ctrlKey || event.metaKey)) return false;
        if (!items.length) return false;
        event.preventDefault();
        choose(items[safeHighlight]);
        return true;
      }
      case "Escape":
        event.preventDefault();
        setDismissed(true);
        return true;
      default:
        return false;
    }
  }

  return {
    open,
    loading,
    error,
    query: query ?? "",
    items,
    highlight: safeHighlight,
    setHighlight,
    choose,
    onKeyDown,
  };
}
