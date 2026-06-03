import { useEffect, useState } from "preact/hooks";
import "@xterm/xterm/css/xterm.css";
import type { ChatMeta } from "../../../models/chat";
import { useTerminalSession } from "../../../hooks/chat/useTerminalSession";
import { Terminal as TerminalIcon, X } from "../../ui/icons";

export function TerminalOverlay({
  chat,
  open,
  onClose,
}: {
  chat: ChatMeta;
  open: boolean;
  onClose: () => void;
}) {
  const [openedChatId, setOpenedChatId] = useState<string | null>(() => open ? chat.id : null);
  const terminal = useTerminalSession({
    chatId: chat.id,
    enabled: openedChatId === chat.id,
    title: chat.title,
  });
  const [entered, setEntered] = useState(false);

  useEffect(() => {
    if (open) {
      setOpenedChatId(chat.id);
      return;
    }
    setOpenedChatId((current) => current === chat.id ? current : null);
  }, [chat.id, open]);

  useEffect(() => {
    if (!open) {
      setEntered(false);
      return;
    }
    const frame = requestAnimationFrame(() => setEntered(true));
    return () => cancelAnimationFrame(frame);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(terminal.focus);
    return () => cancelAnimationFrame(frame);
  }, [open, terminal.focus]);

  const workspacePath = chat.cwd || "/workspace";
  const statusLabel =
    terminal.status === "connected" ? "Connected" :
    terminal.status === "connecting" ? "Connecting" :
    terminal.status === "error" ? "Error" :
    "Closed";

  return (
    <div
      class={`fixed inset-0 z-50 transition-opacity duration-200
              ${open ? "pointer-events-auto" : "pointer-events-none"}
              ${entered ? "opacity-100" : "opacity-0"}`}
      aria-hidden={!open}
    >
      <button
        type="button"
        class="absolute inset-0 bg-black/60"
        onClick={onClose}
        aria-label="Close terminal"
        tabIndex={open ? 0 : -1}
      />
      <aside
        class={`absolute inset-y-0 right-0 w-full sm:w-[min(88vw,920px)] bg-[#0f1014]
                border-l border-white/10 shadow-2xl flex flex-col min-h-0
                transition-transform duration-200 ease-out
                ${entered ? "translate-x-0" : "translate-x-full"}`}
        role="dialog"
        aria-modal="true"
        aria-label="Workspace terminal"
      >
        <header class="codex-header flex-none bg-[#191a1f] border-b border-white/10 px-3 md:px-4 py-2.5 flex items-center gap-2">
          <div class="h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
            <TerminalIcon class="w-4 h-4 text-accent-blue" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 min-w-0">
              <h2 class="truncate text-[15px] md:text-base font-semibold text-ink-50">
                Terminal
              </h2>
              <span class={`h-2 w-2 rounded-full flex-none ${terminal.status === "connected" ? "bg-accent-green" : "bg-ink-400"}`} />
            </div>
            <div class="truncate text-[12px] text-ink-300">
              {statusLabel} - {workspacePath}
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center"
            title="Close terminal"
            aria-label="Close terminal"
          >
            <X class="w-4 h-4" />
          </button>
        </header>

        {terminal.error && (
          <div class="mx-3 md:mx-4 mt-3 rounded-md border border-accent-red/30 bg-accent-red/10 px-3 py-2 text-sm text-accent-red">
            {terminal.error}
          </div>
        )}

        <div class="flex-1 min-h-0 p-2 md:p-3">
          <div
            ref={terminal.hostRef}
            class="h-full w-full overflow-hidden rounded-md border border-white/10 bg-[#0f1014] p-2"
          />
        </div>
      </aside>
    </div>
  );
}
