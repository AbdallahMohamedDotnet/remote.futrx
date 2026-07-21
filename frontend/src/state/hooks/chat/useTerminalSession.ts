import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal as XTerm, type ITheme } from "@xterm/xterm";
import { terminalApi } from "../../../api/terminalApi";
import type { TerminalConnection, TerminalStatus } from "../../../types/terminal";

const terminalTheme: ITheme = {
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
};

function createTerminal(): XTerm {
  return new XTerm({
    cursorBlink: true,
    convertEol: true,
    fontFamily: "ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace",
    fontSize: 13,
    lineHeight: 1.18,
    scrollback: 6000,
    theme: terminalTheme,
  });
}

export function useTerminalSession({
  chatId,
  enabled,
  title,
}: {
  chatId: string;
  enabled: boolean;
  title?: string;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const connectionRef = useRef<TerminalConnection | null>(null);
  const titleRef = useRef(title);
  const [status, setStatus] = useState<TerminalStatus>("closed");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    titleRef.current = title;
  }, [title]);

  const fitAndResize = useCallback(() => {
    const terminal = terminalRef.current;
    const fit = fitRef.current;
    const connection = connectionRef.current;
    if (!terminal || !fit) return;
    try {
      fit.fit();
      if (connection?.isOpen) {
        connection.resize(terminal.cols, terminal.rows);
      }
    } catch {}
  }, []);

  const focus = useCallback(() => {
    terminalRef.current?.focus();
    fitAndResize();
  }, [fitAndResize]);

  useEffect(() => {
    if (!enabled) {
      setStatus("closed");
      setError(null);
      return;
    }
    if (!hostRef.current) return;

    let disposed = false;
    const terminal = createTerminal();
    const fit = new FitAddon();
    terminal.loadAddon(fit);
    terminal.open(hostRef.current);
    terminalRef.current = terminal;
    fitRef.current = fit;

    const connection = terminalApi.connect(chatId, {
      onOpen() {
        if (disposed) return;
        setStatus("connected");
        terminal.writeln(`Connected to ${titleRef.current || "workspace"} terminal`);
        terminal.writeln("");
        fitAndResize();
      },
      onOutput(data) {
        if (disposed) return;
        terminal.write(data);
      },
      onError() {
        if (disposed) return;
        setError("Terminal connection failed.");
        setStatus("error");
      },
      onClose() {
        if (disposed) return;
        setStatus((current) => current === "error" ? current : "closed");
      },
    });
    connectionRef.current = connection;
    setStatus("connecting");
    setError(null);

    const inputSub = terminal.onData((data) => {
      if (connection.isOpen) {
        connection.sendInput(data);
      }
    });

    const resizeObserver = new ResizeObserver(() => {
      if (connection.isOpen) fitAndResize();
    });
    resizeObserver.observe(hostRef.current);
    window.setTimeout(fitAndResize, 0);

    return () => {
      disposed = true;
      resizeObserver.disconnect();
      inputSub.dispose();
      connection.close();
      terminal.dispose();
      connectionRef.current = null;
      terminalRef.current = null;
      fitRef.current = null;
    };
  }, [chatId, enabled, fitAndResize]);

  return {
    hostRef,
    status,
    error,
    focus,
  };
}
