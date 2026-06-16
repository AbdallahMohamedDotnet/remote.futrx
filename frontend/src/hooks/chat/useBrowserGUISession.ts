import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { browserGUIWebSocketUrl } from "../../api/websocket";
import { buildGuiUrl } from "../../components/chat/browser/browserUrls";

export type BrowserGUIStatus = "idle" | "starting" | "ready" | "error" | "stopped";

interface ServerMsg {
  type?: string;
  slug?: string;
  port?: number;
  message?: string;
}

// useBrowserGUISession owns the /ws/browser-gui control channel: it asks the
// backend to bring up the in-container Agent Browser and tracks its status.
// Pixels do NOT flow here — once ready, the noVNC view loads as an iframe
// straight from the dev-URL proxy. Closing the channel (toggling the pane off)
// deliberately does NOT stop the browser server-side, since the agent may
// still be driving it; the explicit stop() does.
export function useBrowserGUISession({ chatId, enabled }: { chatId: string; enabled: boolean }) {
  const socketRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<BrowserGUIStatus>("idle");
  const [guiUrl, setGuiUrl] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!enabled || !chatId) {
      setStatus("idle");
      setGuiUrl("");
      setError(null);
      return;
    }

    let disposed = false;
    const socket = new WebSocket(browserGUIWebSocketUrl(chatId));
    socketRef.current = socket;
    setStatus("starting");
    setGuiUrl("");
    setError(null);

    socket.onmessage = (event) => {
      if (disposed) return;
      let msg: ServerMsg;
      try {
        msg = JSON.parse(String(event.data));
      } catch {
        return;
      }
      switch (msg.type) {
        case "starting":
          setStatus("starting");
          break;
        case "ready": {
          const next = buildGuiUrl(msg.slug ?? "", msg.port ?? 0);
          if (!next) {
            setError("Agent browser started but returned an incomplete address.");
            setStatus("error");
            break;
          }
          setGuiUrl(next);
          setStatus("ready");
          break;
        }
        case "stopped":
          setGuiUrl("");
          setStatus("stopped");
          break;
        case "error":
          setError(msg.message || "Failed to start the agent browser.");
          setStatus("error");
          break;
      }
    };
    socket.onerror = () => {
      if (disposed) return;
      setError("Agent browser connection failed.");
      setStatus("error");
    };
    // A closed control channel leaves the browser running (the noVNC iframe
    // keeps its own connection), so a ready session stays usable.
    socket.onclose = () => {
      if (disposed) return;
      setStatus((current) => (current === "ready" || current === "error" ? current : "idle"));
    };

    return () => {
      disposed = true;
      try {
        socket.close();
      } catch {}
      socketRef.current = null;
    };
  }, [chatId, enabled]);

  const stop = useCallback(() => {
    const socket = socketRef.current;
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type: "stop" }));
    }
  }, []);

  return { status, guiUrl, error, stop };
}
