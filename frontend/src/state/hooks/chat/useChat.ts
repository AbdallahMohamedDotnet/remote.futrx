import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { chatApi } from "../../../api/chatApi";
import type { ChatStream } from "../../../types/chatApi";
import type { ChatEvent, ChatEventPage, ChatMeta, ChatStatus } from "../../../models/chat";
import { groupEvents, type Block } from "../../chat/messageBlocks";
import {
  addUsageFromEvent,
  EMPTY_USAGE_TOTALS,
  type UsageTotals,
} from "../../chat/usage";

const CHAT_EVENT_PAGE_LIMIT = 240;

interface UseChatResult {
  meta: ChatMeta | null;
  blocks: Block[];
  usageTotals: UsageTotals;
  eventCount: number;
  hasOlder: boolean;
  loadingOlder: boolean;
  status: ChatStatus;
  error: string | null;
  canSendPrompt: boolean;
  sendPrompt: (text: string) => boolean;
  cancel: () => void;
  rewind: (beforeT: number) => Promise<ChatEventPage>;
  loadOlder: () => Promise<void>;
  refreshMeta: () => Promise<void>;
}

interface ChatRenderState {
  events: ChatEvent[];
  blocks: Block[];
  usageTotals: UsageTotals;
  eventCount: number;
  hasOlder: boolean;
  nextBefore: number;
  lastSeq: number;
}

function emptyChatRenderState(): ChatRenderState {
  return {
    events: [],
    blocks: [],
    usageTotals: EMPTY_USAGE_TOTALS,
    eventCount: 0,
    hasOlder: false,
    nextBefore: 0,
    lastSeq: 0,
  };
}

function stateFromEvents(
  events: ChatEvent[],
  page: Pick<ChatEventPage, "hasMore" | "nextBefore" | "lastSeq">
): ChatRenderState {
  let usageTotals = EMPTY_USAGE_TOTALS;

  for (const event of events) {
    usageTotals = addUsageFromEvent(usageTotals, event);
  }

  return {
    events,
    blocks: groupEvents(events),
    usageTotals,
    eventCount: events.length,
    hasOlder: page.hasMore,
    nextBefore: page.nextBefore ?? 0,
    lastSeq: Math.max(page.lastSeq, latestSeq(events)),
  };
}

function appendChatEvents(state: ChatRenderState, events: ChatEvent[]): ChatRenderState {
  if (events.length === 0) return state;
  const merged = mergeEvents(state.events, events);
  return stateFromEvents(merged, {
    hasMore: state.hasOlder,
    nextBefore: state.nextBefore,
    lastSeq: Math.max(state.lastSeq, latestSeq(events)),
  });
}

function prependChatPage(state: ChatRenderState, page: ChatEventPage): ChatRenderState {
  const merged = mergeEvents(page.events, state.events);
  return stateFromEvents(merged, page);
}

function mergeEvents(a: ChatEvent[], b: ChatEvent[]): ChatEvent[] {
  const merged = [...a];
  const seenSeqs = new Set<number>();
  for (const event of merged) {
    if (event.seq) seenSeqs.add(event.seq);
  }
  for (const event of b) {
    if (event.seq && seenSeqs.has(event.seq)) continue;
    merged.push(event);
    if (event.seq) seenSeqs.add(event.seq);
  }
  return merged.sort((left, right) => eventOrder(left) - eventOrder(right));
}

function eventOrder(event: ChatEvent): number {
  return event.seq || event.t;
}

function latestSeq(events: ChatEvent[]): number {
  return events.reduce((max, event) => Math.max(max, event.seq || 0), 0);
}

function statusAfterEvent(event: ChatEvent, current: ChatStatus): ChatStatus {
  if (event.type === "complete" || event.type === "error") {
    // The backend clears the run lock in a later sync event. Keep streaming
    // until sync running=false so queued prompts are not sent into a locked run.
    return current === "streaming" ? "streaming" : "ready";
  }
  return "streaming";
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
  const [loadingOlder, setLoadingOlder] = useState(false);
  const streamRef = useRef<ChatStream | null>(null);
  const pendingEventsRef = useRef<ChatEvent[]>([]);
  const pendingFrameRef = useRef<number | null>(null);
  const lastSeqRef = useRef(0);

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
    lastSeqRef.current = Math.max(lastSeqRef.current, latestSeq(events));
    setRenderState((current) => appendChatEvents(current, events));
    setStatus((current) => statusAfterEvent(events[events.length - 1], current));
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
    setLoadingOlder(false);
    lastSeqRef.current = 0;

    (async () => {
      try {
        const [m, page] = await Promise.all([
          chatApi.fetch(chatId),
          chatApi.events(chatId, { limit: CHAT_EVENT_PAGE_LIMIT }),
        ]);
        if (cancelled) return;
        lastSeqRef.current = Math.max(page.lastSeq, latestSeq(page.events));
        setRenderState(stateFromEvents(page.events, page));
        setMeta(m);
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

  // Open WS once the latest page is loaded. Reconnects request only events
  // after the latest sequence this client has already applied.
  useEffect(() => {
    if (!meta || meta.id !== chatId) return;
    const streamChatId = meta.id;
    setWsReady(false);

    const stream = chatApi.openStream(
      streamChatId,
      () => lastSeqRef.current,
      {
        onOpen: () => {
          if (streamRef.current !== stream) return;
          setError(null);
          clearPendingEvents();
          setWsReady(true);
        },
        onEvent: (event) => {
          if (streamRef.current !== stream) return;
          if (event.type === "sync") {
            setStatus(event.running ? "streaming" : "ready");
            return;
          }
          enqueueEvent(event);
        },
        onClose: () => {
          if (streamRef.current === stream) setWsReady(false);
        },
      }
    );
    streamRef.current = stream;

    return () => {
      if (streamRef.current === stream) streamRef.current = null;
      setWsReady(false);
      clearPendingEvents();
      stream.close();
    };
  }, [meta?.id, chatId]);

  const sendPrompt = useCallback((text: string) => {
    const stream = streamRef.current;
    if (!wsReady || !stream?.isOpen) return false;
    if (status !== "ready") return false;
    setStatus("streaming");
    stream.sendPrompt(text);
    return true;
  }, [status, wsReady]);

  const cancel = useCallback(() => {
    const stream = streamRef.current;
    if (stream?.isOpen) stream.cancel();
  }, []);

  const rewind = useCallback(async (beforeT: number) => {
    const res = await chatApi.rewind(chatId, beforeT);
    clearPendingEvents();
    lastSeqRef.current = Math.max(res.lastSeq, latestSeq(res.events));
    setRenderState(stateFromEvents(res.events, res));
    setStatus("ready");
    return res;
  }, [chatId]);

  const loadOlder = useCallback(async () => {
    if (loadingOlder || !renderState.hasOlder || !renderState.nextBefore) return;
    setLoadingOlder(true);
    try {
      const page = await chatApi.events(chatId, {
        limit: CHAT_EVENT_PAGE_LIMIT,
        before: renderState.nextBefore,
      });
      setRenderState((current) => prependChatPage(current, page));
    } finally {
      setLoadingOlder(false);
    }
  }, [chatId, loadingOlder, renderState.hasOlder, renderState.nextBefore]);

  const refreshMeta = useCallback(async () => {
    if (!chatId) return;
    try {
      const m = await chatApi.fetch(chatId);
      setMeta(m);
    } catch {}
  }, [chatId]);

  return {
    meta,
    blocks: renderState.blocks,
    usageTotals: renderState.usageTotals,
    eventCount: renderState.eventCount,
    hasOlder: renderState.hasOlder,
    loadingOlder,
    status,
    error,
    canSendPrompt: wsReady && status === "ready",
    sendPrompt,
    cancel,
    rewind,
    loadOlder,
    refreshMeta,
  };
}
