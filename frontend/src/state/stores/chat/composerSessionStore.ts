import type { QueuedPrompt } from "../../../models/chat";
import { SESSION_STORAGE_KEYS } from "../../../config/storageKeys.ts";

const STORAGE_KEY = SESSION_STORAGE_KEYS.composerSession;

type StorageLike = Pick<Storage, "getItem" | "setItem">;

interface PersistedComposerSession {
  drafts: Record<string, string>;
  queues: Record<string, QueuedPrompt[]>;
}

class ChatComposerSessionStore {
  private readonly drafts = new Map<string, string>();
  private readonly promptQueues = new Map<string, QueuedPrompt[]>();
  private readonly storage: StorageLike | null;

  constructor(storage: StorageLike | null = defaultStorage()) {
    this.storage = storage;
    this.hydrate();
  }

  getDraft(chatId: string): string {
    return this.drafts.get(chatId) ?? "";
  }

  setDraft(chatId: string, text: string): void {
    if (text) this.drafts.set(chatId, text);
    else this.drafts.delete(chatId);
    this.persist();
  }

  getQueuedPrompts(chatId: string): QueuedPrompt[] {
    return this.promptQueues.get(chatId) ?? [];
  }

  setQueuedPrompts(chatId: string, prompts: QueuedPrompt[]): void {
    if (prompts.length) this.promptQueues.set(chatId, prompts);
    else this.promptQueues.delete(chatId);
    this.persist();
  }

  // Composer state is mirrored to sessionStorage so drafts and queued prompts
  // survive a reload or navigation while an agent run (often a long stretch of
  // tool calls) is still in flight. sessionStorage keeps it per-tab, matching
  // the previous in-memory semantics; storage failures degrade to memory-only.
  private hydrate(): void {
    if (!this.storage) return;
    try {
      const raw = this.storage.getItem(STORAGE_KEY);
      if (!raw) return;
      const parsed = JSON.parse(raw) as Partial<PersistedComposerSession> | null;
      for (const [chatId, text] of Object.entries(parsed?.drafts ?? {})) {
        if (typeof text === "string" && text) this.drafts.set(chatId, text);
      }
      for (const [chatId, prompts] of Object.entries(parsed?.queues ?? {})) {
        const valid = (Array.isArray(prompts) ? prompts : []).filter(
          (prompt): prompt is QueuedPrompt =>
            !!prompt && typeof prompt.id === "string" && typeof prompt.text === "string",
        );
        if (valid.length) this.promptQueues.set(chatId, valid);
      }
    } catch {
      // Corrupt or unreadable snapshot — start clean.
    }
  }

  private persist(): void {
    if (!this.storage) return;
    try {
      const snapshot: PersistedComposerSession = {
        drafts: Object.fromEntries(this.drafts),
        queues: Object.fromEntries(this.promptQueues),
      };
      this.storage.setItem(STORAGE_KEY, JSON.stringify(snapshot));
    } catch {
      // Quota or privacy-mode failures fall back to in-memory behavior.
    }
  }
}

function defaultStorage(): StorageLike | null {
  try {
    return typeof window === "undefined" ? null : window.sessionStorage;
  } catch {
    return null;
  }
}

export { ChatComposerSessionStore };

// ChatContainer remounts when the active chat changes, so composer state must
// outlive the component tree. Backed by per-tab sessionStorage so it also
// survives reloads within the browser session.
export const chatComposerSessionStore = new ChatComposerSessionStore();
