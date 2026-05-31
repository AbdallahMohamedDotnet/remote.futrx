import { useEffect, useRef, useState } from "preact/hooks";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { terminalWebSocketUrl } from "../../api/websocket";
import type { ChatMeta } from "../../models/chat";
import { Terminal as TerminalIcon, X } from "../ui/icons";

type TerminalStatus = "connecting" | "connected" | "closed" | "error";

export function TerminalOverlay({
  chat,
  open,
  onClose,
}: {
  chat: ChatMeta;
  open: boolean;
  onClose: () => void;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<TerminalStatus>("closed");
  const [error, setError] = useState<string | null>(null);
  const [entered, setEntered] = useState(false);

  useEffect(() => {
    if (!open) {
      setEntered(false);
      return;
    }
    const frame = requestAnimationFrame(() => setEntered(true));
    return () => cancelAnimationFrame(frame);
  }, [open]);

  useEffect(() => {
    if (!open || !hostRef.current) return;

    const terminal = new XTerm({
      cursorBlink: true,
      convertEol: true,
      fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace",
      fontSize: 13,
      lineHeight: 1.18,
      scrollback: 6000,
      theme: {
        background: "#0f1014",
        foreground: "#e4e4e7",
        cursor: "#f4f4f5",
        selectionBackground: "#3f4047",
        black: "#18191e",
        red: "#ff7b72",
        green: "#7bd88f",
        yellow: "#e2b86d",
        blue: "#8ab4ff",
        magenta: "#b8a8ff",
        cyan: "#7dd3fc",
        white: "#e4e4e7",
        brightBlack: "#707078",
        brightRed: "#ff9b96",
        brightGreen: "#b7f7c2",
        brightYellow: "#f0d28a",
        brightBlue: "#a7c7ff",
        brightMagenta: "#c8bbff",
        brightCyan: "#a5f3fc",
        brightWhite: "#f4f4f5",
      },
    });
    const fit = new FitAddon();
    terminal.loadAddon(fit);
    terminal.open(hostRef.current);
    terminal.focus();

    const socket = new WebSocket(terminalWebSocketUrl(chat.id));
    socket.binaryType = "arraybuffer";
    setStatus("connecting");
    setError(null);

    const sendResize = () => {
      try {
        fit.fit();
        socket.send(JSON.stringify({ type: "resize", cols: terminal.cols, rows: terminal.rows }));
      } catch {}
    };

    socket.onopen = () => {
      setStatus("connected");
      terminal.writeln(`Connected to ${chat.title || "workspace"} terminal`);
      terminal.writeln("");
      sendResize();
    };
    socket.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        terminal.write(new Uint8Array(event.data));
        return;
      }
      terminal.write(String(event.data));
    };
    socket.onerror = () => {
      setError("Terminal connection failed.");
      setStatus("error");
    };
    socket.onclose = () => {
      setStatus((current) => current === "error" ? current : "closed");
    };

    const inputSub = terminal.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "input", data }));
      }
    });

    const resizeObserver = new ResizeObserver(() => {
      if (socket.readyState === WebSocket.OPEN) sendResize();
    });
    resizeObserver.observe(hostRef.current);
    window.setTimeout(sendResize, 0);

    return () => {
      resizeObserver.disconnect();
      inputSub.dispose();
      try { socket.close(); } catch {}
      terminal.dispose();
    };
  }, [chat.id, chat.title, open]);

  const workspacePath = chat.cwd || "/workspace";
  const statusLabel =
    status === "connected" ? "Connected" :
    status === "connecting" ? "Connecting" :
    status === "error" ? "Error" :
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
              <span class={`h-2 w-2 rounded-full flex-none ${status === "connected" ? "bg-accent-green" : "bg-ink-400"}`} />
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

        {error && (
          <div class="mx-3 md:mx-4 mt-3 rounded-md border border-accent-red/30 bg-accent-red/10 px-3 py-2 text-sm text-accent-red">
            {error}
          </div>
        )}

        <div class="flex-1 min-h-0 p-2 md:p-3">
          <div
            ref={hostRef}
            class="h-full w-full overflow-hidden rounded-md border border-white/10 bg-[#0f1014] p-2"
          />
        </div>
      </aside>
    </div>
  );
}
