import { useEffect, useRef } from "preact/hooks";
import type { ChatStatus } from "../../../models/chat";
import { chatApi } from "../../../api/chatApi";

export function useChatReadMarker({
  chatId,
  eventCount,
  status,
}: {
  chatId: string;
  eventCount: number;
  status: ChatStatus;
}) {
  const readMarkerRef = useRef("");

  useEffect(() => {
    if (status !== "ready") return;
    const key = `${chatId}:${eventCount}`;
    if (readMarkerRef.current === key) return;
    readMarkerRef.current = key;
    void chatApi.markRead(chatId).catch(() => {});
  }, [chatId, eventCount, status]);
}
