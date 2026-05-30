import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { chatWebSocketUrl } from "../../api/websocket";
import { chatService } from "../../services/chatService";
import type { ChatEvent, ChatMeta, ChatStatus } from "../../models/chat";
import { appendEventToBlocks, type Block } from "../../state/chat/messageBlocks";
import {
  addUsageFromEvent,
  EMPTY_USAGE_TOTALS,
  type UsageTotals,
} from "../../state/chat/usage";

interface UseChatResult {
  meta: ChatMeta | null;
  blocks: Block[];
  usageTotals: UsageTotals;
  eventCount: number;
  status: ChatStatus;
  error: string | null;
  canSendPrompt: boolean;
  sendPrompt: (text: string) => boolean;
  cancel: () => void;
  rewind: (beforeT: number) => Promise<ChatEvent[]>;
  refreshMeta: () => Promise<void>;
}

interface ChatRenderState {
  blocks: Block[];
  usageTotals: UsageTotals;
  eventCount: number;
}

function emptyChatRenderState(): ChatRenderState {
  return { blocks: [], usageTotals: EMPTY_USAGE_TOTALS, eventCount: 0 };
}

function applyChatEvents(state: ChatRenderState, events: ChatEvent[]): ChatRenderState {
  let blocks = state.blocks;
  let usageTotals = state.usageTotals;
  let eventCount = state.eventCount;

  for (const event of events) {
    blocks = appendEventToBlocks(blocks, event);
    usageTotals = addUsageFromEvent(usageTotals, event);
    eventCount++;
  }

  return { blocks, usageTotals, eventCount };
}

function statusAfterEvent(event: ChatEvent): ChatStatus {
  return event.type === "complete" || event.type === "error" ? "ready" : "streaming";
}

/**
 * useChat — load chat metadata, then open a streaming WS. The server replays
 * history over the socket before live events, which avoids gaps between an
 * HTTP history fetch and the WS subscription.
 */
export function useChat(chatId: string): UseChatResult {
  const [meta, setMeta] = useState<ChatMeta | null>(null);
  const [renderState, setRenderState] = useState<ChatRenderState>(() => emptyChatRenderState());
  const [status, setStatus] = useState<ChatStatus>("loading");
  const [error, setError] = useState<string | null>(null);
  const [wsReady, setWsReady] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const pendingEventsRef = useRef<ChatEvent[]>([]);
  const pendingFrameRef = useRef<number | null>(null);

  function clearPendingEvents() {
    if (pendingFrameRef.current !== null) {
      cancelAnimationFrame(pendingFrameRef.current);
      pendingFrameRef.current = null;
    }
    pendingEventsRef.current = [];
  }

  function flushPendingEvents() {
    pendingFrameRef.current = null;
    const events = pendingEventsRef.current;
    if (events.length === 0) return;
    pendingEventsRef.current = [];
    setRenderState((current) => applyChatEvents(current, events));
    setStatus(statusAfterEvent(events[events.length - 1]));
  }

  function enqueueEvent(event: ChatEvent) {
    pendingEventsRef.current.push(event);
    if (pendingFrameRef.current === null) {
      pendingFrameRef.current = requestAnimationFrame(flushPendingEvents);
    }
  }

  // Load metadata when chat id changes.
  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    clearPendingEvents();
    setRenderState(emptyChatRenderState());
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
        clearPendingEvents();
        setRenderState(emptyChatRenderState());
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
          enqueueEvent(ev);
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
      clearPendingEvents();
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
    clearPendingEvents();
    setRenderState(applyChatEvents(emptyChatRenderState(), res.events));
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
    blocks: renderState.blocks,
    usageTotals: renderState.usageTotals,
    eventCount: renderState.eventCount,
    status,
    error,
    canSendPrompt: wsReady && status === "ready",
    sendPrompt,
    cancel,
    rewind,
    refreshMeta,
  };
}
