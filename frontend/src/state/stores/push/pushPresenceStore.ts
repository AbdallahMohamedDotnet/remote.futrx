// Tells the server which chat this user has on screen.
//
// The service worker already stays quiet about a chat it can see for itself,
// but that only covers the browser it runs in: the phone in your pocket has no
// idea you are reading the answer on a laptop. This is the half of the signal
// that travels, and it is what keeps every other device quiet while you are in
// the conversation.
//
// It reports regardless of whether *this* device is subscribed to push — the
// laptop you are typing on may have no subscription at all, and it is still
// the reason the phone should stay silent.

import { pushApi } from "../../../api/pushApi";
import { PUSH_PRESENCE_HEARTBEAT_MS } from "../../../config/push";
import type {
  PushPresenceStoreActions,
  PushPresenceStoreState,
} from "../../../models/push";
import { createAppStore } from "../appStore.ts";
import { isPushPageFocused } from "./pushPageFocus";

const clientId = createClientId();
let heartbeatTimer: number | undefined;
let isListening = false;

/**
 * Keeps the server's idea of what this client is watching in step with what is
 * actually on screen. The claim and its heartbeat change together in one
 * place, so a repeat can never outlive the claim it was repeating.
 */
export const pushPresenceStore = createAppStore<
  PushPresenceStoreState,
  PushPresenceStoreActions
>(
  {
    onScreenChatId: null,
    claimedChatId: null,
    revision: 0,
  },
  ({ getState, setState }) => {
    /**
     * Reports the chat on screen, or null when the app shows something else.
     * Safe to repeat: only a changed claim talks to the server.
     */
    function setWatchedChat(onScreenChatId: string | null): void {
      setState({ onScreenChatId });
      listen();
      sync();
    }

    /** The chat the user counts as watching: in the app, and looking at it. */
    function chatInFocus(): string | null {
      const { onScreenChatId } = getState();
      if (!onScreenChatId || typeof document === "undefined") return null;
      // A visible but unfocused window is one the user left behind for another
      // app, which is exactly when they do want the notification.
      return isPushPageFocused() ? onScreenChatId : null;
    }

    const sync = (): void => {
      claim(chatInFocus());
    };

    /**
     * The only place the claim changes. Restarting the heartbeat here is what
     * keeps "a claim is being repeated" and "there is a claim" the same fact.
     */
    function claim(chatId: string | null): void {
      if (chatId === getState().claimedChatId) return;
      setState({ claimedChatId: chatId });
      restartHeartbeat();
      // Withdrawals ride keepalive: they often fire as the page is going away,
      // and a cancelled one would leave the user silenced until the claim
      // expires.
      void send(chatId, chatId === null);
    }

    function restartHeartbeat(): void {
      if (heartbeatTimer !== undefined) {
        clearInterval(heartbeatTimer);
        heartbeatTimer = undefined;
      }
      if (!getState().claimedChatId) return;
      heartbeatTimer = window.setInterval(() => {
        const { claimedChatId } = getState();
        if (claimedChatId) void send(claimedChatId, false);
      }, PUSH_PRESENCE_HEARTBEAT_MS);
    }

    async function send(chatId: string | null, keepalive: boolean): Promise<void> {
      const revision = getState().revision + 1;
      setState({ revision });
      try {
        await pushApi.presence(
          { chatId: chatId ?? "", clientId, revision },
          keepalive
        );
      } catch {
        // A lost heartbeat costs one notification the user did not need, never
        // one they did, so there is nothing here worth surfacing or retrying.
      }
    }

    function listen(): void {
      if (isListening || typeof window === "undefined") return;
      isListening = true;

      document.addEventListener("visibilitychange", sync);
      window.addEventListener("focus", sync);
      window.addEventListener("blur", sync);
      // The last beat that reliably fires on mobile, where a backgrounded tab
      // may simply never be resumed.
      window.addEventListener("pagehide", () => claim(null));
    }

    return { setWatchedChat };
  },
);

/** Identifies this client for the life of the page. */
function createClientId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}
