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

import { pushApi } from "../../api/pushApi";

// How often to repeat a standing claim. Comfortably inside the server's
// expiry, so one dropped request does not un-mark the user.
const HEARTBEAT_MS = 20_000;

/**
 * Keeps the server's idea of what this client is watching in step with what is
 * actually on screen. The claim and its heartbeat change together in one
 * place, so a repeat can never outlive the claim it was repeating.
 */
class PresenceReporter {
  /** Identifies this client for the life of the page. */
  readonly #clientId = newClientId();
  /** The chat this client is showing, whether or not the user is looking. */
  #onScreen: string | null = null;
  /** The claim the server currently believes, so repeats stay cheap. */
  #claimed: string | null = null;
  #heartbeat: number | undefined;
  #listening = false;

  /** Reports the chat on screen, or null when the app shows something else. */
  report(chatId: string | null) {
    this.#onScreen = chatId;
    this.#listen();
    this.#sync();
  }

  /** The chat the user counts as watching: in the app, and looking at it. */
  #chatInFocus(): string | null {
    if (!this.#onScreen || typeof document === "undefined") return null;
    // A visible but unfocused window is one the user left behind for another
    // app, which is exactly when they do want the notification.
    if (document.visibilityState !== "visible" || !document.hasFocus()) return null;
    return this.#onScreen;
  }

  #sync = () => {
    this.#claim(this.#chatInFocus());
  };

  /**
   * The only place the claim changes. Restarting the heartbeat here is what
   * keeps "a claim is being repeated" and "there is a claim" the same fact.
   */
  #claim(chatId: string | null) {
    if (chatId === this.#claimed) return;
    this.#claimed = chatId;
    this.#restartHeartbeat();
    // Withdrawals ride keepalive: they often fire as the page is going away,
    // and a cancelled one would leave the user silenced until the claim
    // expires.
    void this.#send(chatId, chatId === null);
  }

  #restartHeartbeat() {
    if (this.#heartbeat !== undefined) {
      clearInterval(this.#heartbeat);
      this.#heartbeat = undefined;
    }
    if (!this.#claimed) return;
    this.#heartbeat = window.setInterval(() => {
      if (this.#claimed) void this.#send(this.#claimed, false);
    }, HEARTBEAT_MS);
  }

  async #send(chatId: string | null, keepalive: boolean) {
    try {
      await pushApi.presence({ chatId: chatId ?? "", clientId: this.#clientId }, keepalive);
    } catch {
      // A lost heartbeat costs one notification the user did not need, never
      // one they did, so there is nothing here worth surfacing or retrying.
    }
  }

  #listen() {
    if (this.#listening || typeof window === "undefined") return;
    this.#listening = true;

    document.addEventListener("visibilitychange", this.#sync);
    window.addEventListener("focus", this.#sync);
    window.addEventListener("blur", this.#sync);
    // The last beat that reliably fires on mobile, where a backgrounded tab
    // may simply never be resumed.
    window.addEventListener("pagehide", () => this.#claim(null));
  }
}

const reporter = new PresenceReporter();

/**
 * Reports the chat on screen, or null when the app is showing something else.
 * Safe to call on every render: it only talks to the server when the claim
 * actually changes.
 */
export function setWatchedChat(chatId: string | null) {
  reporter.report(chatId);
}

function newClientId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}
