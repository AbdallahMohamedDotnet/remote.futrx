// The page's half of the service worker conversation.
//
// Two messages flow between them:
//   which-chat  SW -> page, before showing a notification. The page answers
//               with the chat it is displaying so the worker can stay quiet
//               about a chat the user is already watching.
//   open-chat   page <- SW, when a notification is tapped.

import { pushServiceWorkerApi } from "../../api/pushServiceWorkerApi";

type ChatOpener = (chatId: string | null) => void;

/** The chat currently on screen, or null when the window is not focused. */
let visibleChatId: string | null = null;
let openChat: ChatOpener | null = null;
let listening = false;

export function serviceWorkerSupported(): boolean {
  return pushServiceWorkerApi.isSupported;
}

/**
 * Registers the worker and returns its registration. Resolves to null when the
 * browser has no service worker support, so callers can degrade quietly.
 */
export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!serviceWorkerSupported()) return null;
  listen();
  return pushServiceWorkerApi.register();
}

/** Reports which chat is on screen, so the worker can suppress its notification. */
export function setVisibleChat(chatId: string | null) {
  visibleChatId = chatId;
}

/** Routes notification taps back into the app's chat selection. */
export function onNotificationOpen(handler: ChatOpener) {
  openChat = handler;
  listen();
}

function listen() {
  if (listening || !serviceWorkerSupported()) return;
  listening = true;

  pushServiceWorkerApi.listen({
    visibleChatId: () => {
      // Only claim a chat when this window is genuinely in front; a
      // background tab showing the chat should still raise a notification.
      return document.visibilityState === "visible" && document.hasFocus()
        ? visibleChatId
        : null;
    },
    openChat: (chatId) => openChat?.(chatId),
  });
}

/**
 * Reads a chat id handed over by a cold-start notification tap and clears it
 * from the address bar, so reloading later does not jump back to it.
 */
export function takeRequestedChatId(): string | null {
  if (typeof window === "undefined") return null;
  const params = new URLSearchParams(window.location.search);
  const chatId = params.get("chat");
  if (!chatId) return null;

  params.delete("chat");
  const query = params.toString();
  window.history.replaceState(
    null,
    "",
    window.location.pathname + (query ? `?${query}` : "") + window.location.hash
  );
  return chatId;
}
