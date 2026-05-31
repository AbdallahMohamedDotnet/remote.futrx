import { useEffect, useRef, useState } from "preact/hooks";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { terminalWebSocketUrl } from "../../api/websocket";
import { chatService } from "../../services/chatService";
import type { ChatMeta } from "../../models/chat";
import { Code, Terminal as TerminalIcon, X } from "../../components/ui/icons";

type TerminalStatus = "loading" | "connecting" | "connected" | "closed" | "error";

export function TerminalRoute({ chatId }: { chatId: string }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const termRef = useRef<XTerm | null>(null);
  const [chat, setChat] = useState<ChatMeta | null>(null);
  const [status, setStatus] = useState<TerminalStatus>("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setError(null);
    chatService.get(chatId)
      .then((next) => {
        if (!cancelled) setChat(next);
      })
      .catch((err) => {
        if (!cancelled) {
          setError((err as Error).message);
          setStatus("error");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [chatId]);

  useEffect(() => {
    if (!chat || !hostRef.current) return;

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
    termRef.current = terminal;
    fitRef.current = fit;

    const socket = new WebSocket(terminalWebSocketUrl(chat.id));
    socket.binaryType = "arraybuffer";
    socketRef.current = socket;
    setStatus("connecting");

    const sendResize = () => {
      try {
        fit.fit();
        socket.send(JSON.stringify({ type: "resize", cols: terminal.cols, rows: terminal.rows }));
      } catch {}
    };

    socket.onopen = () => {
      setStatus("connected");
      setError(null);
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
      socketRef.current = null;
      termRef.current = null;
      fitRef.current = null;
    };
  }, [chat]);

  const workspacePath = chat?.cwd || "/workspace";
  const statusLabel =
    status === "connected" ? "Connected" :
    status === "connecting" ? "Connecting" :
    status === "loading" ? "Loading" :
    status === "error" ? "Error" :
    "Closed";

  return (
    <div class="app-shell codex-app h-full min-h-0 bg-[#0f1014] text-ink-100 flex flex-col">
      <header class="codex-header flex-none bg-[#191a1f] border-b border-white/10 px-3 md:px-4 py-2.5 flex items-center gap-2">
        <div class="h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
          <TerminalIcon class="w-4 h-4 text-accent-blue" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 min-w-0">
            <h1 class="truncate text-[15px] md:text-base font-semibold text-ink-50">
              {chat?.title || "Terminal"}
            </h1>
            <span class={`h-2 w-2 rounded-full flex-none ${status === "connected" ? "bg-accent-green" : "bg-ink-400"}`} />
          </div>
          <div class="truncate text-[12px] text-ink-300">
            {statusLabel} - {workspacePath}
          </div>
        </div>
        <a
          href="/"
          class="h-9 inline-flex items-center gap-2 px-3 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200"
          title="Back to chat"
        >
          <Code class="w-4 h-4 text-accent-blue" />
          <span class="hidden sm:inline text-[12.5px] font-medium">Chat</span>
        </a>
        <button
          type="button"
          onClick={() => window.close()}
          class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center"
          title="Close terminal"
        >
          <X class="w-4 h-4" />
        </button>
      </header>

      {error && (
        <div class="mx-3 md:mx-4 mt-3 rounded-md border border-accent-red/30 bg-accent-red/10 px-3 py-2 text-sm text-accent-red">
          {error}
        </div>
      )}

      <main class="flex-1 min-h-0 p-2 md:p-3">
        <div
          ref={hostRef}
          class="h-full w-full overflow-hidden rounded-md border border-white/10 bg-[#0f1014] p-2"
        />
      </main>
    </div>
  );
}
