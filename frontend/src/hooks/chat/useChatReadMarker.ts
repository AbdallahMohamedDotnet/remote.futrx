import { useEffect, useRef } from "preact/hooks";
import type { ChatStatus } from "../../models/chat";
import { chatService } from "../../api/chatService";

export function useChatReadMarker({
  chatId,
  eventCount,
  status,
  onMetaUpdate,
}: {
  chatId: string;
  eventCount: number;
  status: ChatStatus;
  onMetaUpdate: () => void;
}) {
  const readMarkerRef = useRef("");

  useEffect(() => {
    if (status !== "ready") return;
    const key = `${chatId}:${eventCount}`;
    if (readMarkerRef.current === key) return;
    readMarkerRef.current = key;
    void chatService.markRead(chatId).then(onMetaUpdate).catch(() => {});
  }, [chatId, eventCount, onMetaUpdate, status]);
}
