import type { QueuedPrompt } from "../../models/chat";

// Per-chat composer state — draft text and queued prompts — held outside the
// component tree.
//
// ChatContainer is mounted with `key={chatId}` (see WorkspaceContainer), so
// switching chats fully unmounts and remounts it, destroying all of its hook
// state. If the draft/queue lived only in component state they would be lost
// the moment you leave a chat. Keying this store by chatId lets the composer
// restore exactly what you had when you return.
//
// Intentionally in-memory only: this is ephemeral session state, not persisted
// across page reloads.

const drafts = new Map<string, string>();
const queues = new Map<string, QueuedPrompt[]>();

export function getDraft(chatId: string): string {
  return drafts.get(chatId) ?? "";
}

export function setDraft(chatId: string, text: string): void {
  // Drop empty drafts so the map doesn't accumulate a key per visited chat.
  if (text) drafts.set(chatId, text);
  else drafts.delete(chatId);
}

export function getQueuedPrompts(chatId: string): QueuedPrompt[] {
  return queues.get(chatId) ?? [];
}

export function setQueuedPrompts(chatId: string, prompts: QueuedPrompt[]): void {
  if (prompts.length) queues.set(chatId, prompts);
  else queues.delete(chatId);
}
