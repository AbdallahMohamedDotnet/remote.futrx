import type { UpdateChatInput } from "../../models/chat";
import { chatService } from "../../api/chatService";

export function useChatMetaActions({
  chatId,
  refreshMeta,
  onMetaUpdate,
}: {
  chatId: string;
  refreshMeta: () => Promise<void>;
  onMetaUpdate: () => void;
}) {
  async function applyMeta(patch: UpdateChatInput) {
    try {
      await chatService.update(chatId, patch);
      await refreshMeta();
      onMetaUpdate();
    } catch (error) {
      alert("update failed: " + (error as Error).message);
    }
  }

  return { applyMeta };
}
