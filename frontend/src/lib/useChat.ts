import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { chatsApi } from "./api";
import type { ChatEvent, ChatMeta } from "../types";

export type ChatStatus = "loading" | "ready" | "streaming" | "error";

interface UseChatResult {
  meta: ChatMeta | null;
  events: ChatEvent[];
  status: ChatStatus;
  error: string | null;
  sendPrompt: (text: string) => void;
  cancel: () => void;
  refreshMeta: () => Promise<void>;
}

/**
 * useChat — load an existing chat by id, replay its history, then open a
 * streaming WS for live events. Replays only on chatId change so model /
 * cwd updates don't trigger a full reload.
 */
export function useChat(chatId: string): UseChatResult {
  const [meta, setMeta] = useState<ChatMeta | null>(null);
  const [events, setEvents] = useState<ChatEvent[]>([]);
  const [status, setStatus] = useState<ChatStatus>("loading");
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  // Load history when chat id changes.
  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    setEvents([]);
    setMeta(null);
    setError(null);

    (async () => {
      try {
        const [m, past] = await Promise.all([
          chatsApi.get(chatId),
          chatsApi.events(chatId),
        ]);
        if (cancelled) return;
        setMeta(m);
        setEvents(past);
        setStatus("ready");
      } catch (e) {
        if (!cancelled) {
          setError((e as Error).message);
          setStatus("error");
        }
      }
    })();

    return () => { cancelled = true; };
  }, [chatId]);

  // Open WS once we have meta. Re-open if chat id changes.
  useEffect(() => {
    if (!meta || meta.id !== chatId) return;
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/ws/chat/${meta.id}`);
    wsRef.current = ws;
    ws.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data) as ChatEvent;
        setEvents((prev) => [...prev, ev]);
        if (ev.type === "complete" || ev.type === "error") setStatus("ready");
      } catch {}
    };
    ws.onclose = () => { wsRef.current = null; };
    return () => {
      wsRef.current = null;
      try { ws.close(); } catch {}
    };
  }, [meta?.id, chatId]);

  const sendPrompt = useCallback((text: string) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    if (status === "streaming") return;
    setStatus("streaming");
    ws.send(JSON.stringify({ type: "prompt", text }));
  }, [status]);

  const cancel = useCallback(() => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "cancel" }));
    }
  }, []);

  const refreshMeta = useCallback(async () => {
    if (!chatId) return;
    try {
      const m = await chatsApi.get(chatId);
      setMeta(m);
    } catch {}
  }, [chatId]);

  return { meta, events, status, error, sendPrompt, cancel, refreshMeta };
}
