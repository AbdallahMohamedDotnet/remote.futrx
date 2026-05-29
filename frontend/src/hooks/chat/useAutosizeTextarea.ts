import { useEffect, useRef } from "preact/hooks";

export function useAutosizeTextarea(value: string, maxHeight = 220) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.style.height = "auto";
    textarea.style.height = Math.min(textarea.scrollHeight, maxHeight) + "px";
  }, [value, maxHeight]);

  function focusInput() {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.focus();
    const end = textarea.value.length;
    textarea.setSelectionRange(end, end);
  }

  return { textareaRef, focusInput };
}
