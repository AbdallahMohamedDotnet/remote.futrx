import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { chatWebSocketUrl } from "../../api/websocket";
import { chatService } from "../../services/chatService";
import type { ChatEvent, ChatMeta, ChatStatus } from "../../models/chat";

interface UseChatResult {
  meta: ChatMeta | null;
  events: ChatEvent[];
  status: ChatStatus;
  error: string | null;
  canSendPrompt: boolean;
  sendPrompt: (text: string) => boolean;
  cancel: () => void;
  rewind: (beforeT: number) => Promise<ChatEvent[]>;
  refreshMeta: () => Promise<void>;
}

/**
 * useChat — load chat metadata, then open a streaming WS. The server replays
 * history over the socket before live events, which avoids gaps between an
 * HTTP history fetch and the WS subscription.
 */
export function useChat(chatId: string): UseChatResult {
  const [meta, setMeta] = useState<ChatMeta | null>(null);
  const [events, setEvents] = useState<ChatEvent[]>([]);
  const [status, setStatus] = useState<ChatStatus>("loading");
  const [error, setError] = useState<string | null>(null);
  const [wsReady, setWsReady] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  // Load metadata when chat id changes.
  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setEvents([]);
    setMeta(null);
    setError(null);
    setWsReady(false);

    (async () => {
      try {
        const m = await chatService.get(chatId);
        if (cancelled) return;
        setMeta(m);
      } catch (e) {
        if (!cancelled) {
          setError((e as Error).message);
          setStatus("error");
        }
      }
    })();

    return () => { cancelled = true; };
  }, [chatId]);

  // Open WS once we have meta. If the connection closes mid-stream, reconnect
  // and let the server replay history so the UI catches up without refresh.
  useEffect(() => {
    if (!meta || meta.id !== chatId) return;
    const wsChatId = meta.id;
    let stopped = false;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;
    setWsReady(false);

    function scheduleReconnect() {
      if (stopped) return;
      const delay = Math.min(5000, 400 * 2 ** attempt);
      attempt++;
      reconnectTimer = setTimeout(connect, delay);
    }

    function connect() {
      if (stopped) return;
      const ws = new WebSocket(chatWebSocketUrl(wsChatId));
      wsRef.current = ws;
      setWsReady(false);

      ws.onopen = () => {
        if (stopped || wsRef.current !== ws) return;
        attempt = 0;
        setError(null);
        setEvents([]);
        setWsReady(true);
      };

      ws.onmessage = (e) => {
        if (stopped || wsRef.current !== ws) return;
        try {
          const ev = JSON.parse(e.data) as ChatEvent;
          if (ev.type === "sync") {
            setStatus(ev.running ? "streaming" : "ready");
            return;
          }
          setEvents((prev) => [...prev, ev]);
          if (ev.type === "complete" || ev.type === "error") {
            setStatus("ready");
          } else {
            setStatus("streaming");
          }
        } catch {}
      };

      ws.onclose = () => {
        if (wsRef.current === ws) {
          wsRef.current = null;
          setWsReady(false);
          scheduleReconnect();
        }
      };

      ws.onerror = () => {
        try { ws.close(); } catch {}
      };
    }

    connect();

    return () => {
      stopped = true;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      const ws = wsRef.current;
      wsRef.current = null;
      setWsReady(false);
      try { ws?.close(); } catch {}
    };
  }, [meta?.id, chatId]);

  const sendPrompt = useCallback((text: string) => {
    const ws = wsRef.current;
    if (!wsReady || !ws || ws.readyState !== WebSocket.OPEN) return false;
    if (status !== "ready") return false;
    setStatus("streaming");
    ws.send(JSON.stringify({ type: "prompt", text }));
    return true;
  }, [status, wsReady]);

  const cancel = useCallback(() => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "cancel" }));
    }
  }, []);

  const rewind = useCallback(async (beforeT: number) => {
    const res = await chatService.rewind(chatId, beforeT);
    setEvents(res.events);
    setStatus("ready");
    return res.events;
  }, [chatId]);

  const refreshMeta = useCallback(async () => {
    if (!chatId) return;
    try {
      const m = await chatService.get(chatId);
      setMeta(m);
    } catch {}
  }, [chatId]);

  return {
    meta,
    events,
    status,
    error,
    canSendPrompt: wsReady && status === "ready",
    sendPrompt,
    cancel,
    rewind,
    refreshMeta,
  };
}
