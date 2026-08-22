import { useEffect, useMemo, useState } from "preact/hooks";
import type { ChatProvider } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";
import { useAvailableSkills } from "./useAvailableSkills";

// The palette is only meant to trigger while the composer holds a bare command
// token: a leading "/" followed by no whitespace. As soon as the user types a
// space (i.e. starts writing a real prompt) the trigger falls away.
const SLASH_PATTERN = /^\/(\S*)$/;

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
  const match = SLASH_PATTERN.exec(text);
  const query = match ? match[1] : null;
  const triggered = query !== null;

  const { skills, loading, error } = useAvailableSkills(provider, projectId);
  const [highlight, setHighlight] = useState(0);
  const [dismissed, setDismissed] = useState(false);

  const items = useMemo(() => {
    if (!triggered) return [];
    const term = (query ?? "").trim().toLowerCase();
    if (!term) return skills;
    return skills.filter((skill) =>
      `${skill.name} ${skill.command || ""} ${skill.description || ""} ${skill.source || ""}`
        .toLowerCase()
        .includes(term)
    );
  }, [triggered, query, skills]);

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
  const safeHighlight = items.length ? Math.min(highlight, items.length - 1) : 0;

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
        setHighlight((index) => (items.length ? (index + 1) % items.length : 0));
        return true;
      case "ArrowUp":
        event.preventDefault();
        setHighlight((index) => (items.length ? (index - 1 + items.length) % items.length : 0));
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
