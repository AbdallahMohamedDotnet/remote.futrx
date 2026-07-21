import type { UpdateChatInput } from "../../../models/chat";
import { chatApi } from "../../../api/chatApi";

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
      await chatApi.update(chatId, patch);
      await refreshMeta();
      onMetaUpdate();
    } catch (error) {
      alert("update failed: " + (error as Error).message);
    }
  }

  return { applyMeta };
}
