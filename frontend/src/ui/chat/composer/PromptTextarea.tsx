import type { RefObject } from "preact";

export function PromptTextarea({
  textareaRef,
  text,
  uploading,
  streaming,
  disconnected,
  onTextChange,
  onPaste,
  onSend,
  onKeyDown,
}: {
  textareaRef: RefObject<HTMLTextAreaElement>;
  text: string;
  uploading: boolean;
  streaming: boolean;
  disconnected: boolean;
  onTextChange: (text: string) => void;
  onPaste: (event: ClipboardEvent) => void;
  onSend: () => void;
  // Return true to signal the key was handled (e.g. by the slash palette) so
  // the default composer handling below is skipped.
  onKeyDown?: (event: KeyboardEvent) => boolean;
}) {
  return (
    <textarea
      ref={textareaRef}
      value={text}
      onInput={(event) => onTextChange((event.currentTarget as HTMLTextAreaElement).value)}
      onKeyDown={(event) => {
        if (onKeyDown?.(event)) return;
        if (
          event.key === "Enter" &&
          (event.ctrlKey || event.metaKey) &&
          !event.shiftKey &&
          !event.isComposing
        ) {
          event.preventDefault();
          onSend();
        }
      }}
      onPaste={(event) => onPaste(event as ClipboardEvent)}
      rows={1}
      enterkeyhint="enter"
      aria-keyshortcuts="Control+Enter Meta+Enter"
      autocomplete="off"
      autocapitalize="off"
      autocorrect="off"
      spellcheck={false}
      placeholder={
        uploading ? "Uploading..." :
        streaming ? "Queue next prompt while the agent is working" :
        disconnected ? "Connecting..." :
        "Ask anything, @ to add files, / for commands"
      }
      disabled={disconnected}
      class="codex-composer-textarea flex-1 resize-none rounded-md
             bg-transparent border-0 text-ink-100 placeholder:text-ink-300
             focus:outline-none
             px-2.5 py-2.5 text-[16px] sm:text-[14px] leading-normal
             min-h-[40px] max-h-[220px]
             disabled:opacity-60 transition-colors"
    />
  );
}
