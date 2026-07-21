import type { QueuedPrompt } from "../../models/chat";

class ChatComposerSessionStore {
  private readonly drafts = new Map<string, string>();
  private readonly promptQueues = new Map<string, QueuedPrompt[]>();

  getDraft(chatId: string): string {
    return this.drafts.get(chatId) ?? "";
  }

  setDraft(chatId: string, text: string): void {
    if (text) this.drafts.set(chatId, text);
    else this.drafts.delete(chatId);
  }

  getQueuedPrompts(chatId: string): QueuedPrompt[] {
    return this.promptQueues.get(chatId) ?? [];
  }

  setQueuedPrompts(chatId: string, prompts: QueuedPrompt[]): void {
    if (prompts.length) this.promptQueues.set(chatId, prompts);
    else this.promptQueues.delete(chatId);
  }
}

// ChatContainer remounts when the active chat changes, so composer state must
// outlive the component tree. It remains intentionally in memory for only the
// current browser session.
export const chatComposerSessionStore = new ChatComposerSessionStore();
