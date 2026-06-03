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
}: {
  textareaRef: RefObject<HTMLTextAreaElement>;
  text: string;
  uploading: boolean;
  streaming: boolean;
  disconnected: boolean;
  onTextChange: (text: string) => void;
  onPaste: (event: ClipboardEvent) => void;
  onSend: () => void;
}) {
  return (
    <textarea
      ref={textareaRef}
      value={text}
      onInput={(event) => onTextChange((event.currentTarget as HTMLTextAreaElement).value)}
      onKeyDown={(event) => {
        if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
          event.preventDefault();
          onSend();
        }
      }}
      onPaste={(event) => onPaste(event as ClipboardEvent)}
      rows={1}
      enterkeyhint="send"
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
             bg-[#101318] border border-white/10 text-ink-100 placeholder:text-ink-300
             focus:outline-none focus:border-accent-blue/80 focus:bg-[#121722]
             px-3.5 py-3 text-[16px] sm:text-[14.5px] leading-normal
             min-h-[44px] max-h-[220px] shadow-inner
             disabled:opacity-60 transition-colors"
    />
  );
}
