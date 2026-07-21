import { useEffect } from "preact/hooks";
import type { ChatStatus } from "../../../models/chat";

export function useChatKeyboardShortcuts({
  status,
  onCancel,
}: {
  status: ChatStatus;
  onCancel: () => void;
}) {
  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape" && status === "streaming") onCancel();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [status, onCancel]);
}
