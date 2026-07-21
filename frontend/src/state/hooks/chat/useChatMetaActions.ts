import type { UpdateChatInput } from "../../../models/chat";
import { chatApi } from "../../../api/chatApi";

export function useChatMetaActions({
  chatId,
  refreshMeta,
}: {
  chatId: string;
  refreshMeta: () => Promise<void>;
}) {
  async function applyMeta(patch: UpdateChatInput) {
    try {
      await chatApi.update(chatId, patch);
      await refreshMeta();
    } catch (error) {
      alert("update failed: " + (error as Error).message);
    }
  }

  return { applyMeta };
}
