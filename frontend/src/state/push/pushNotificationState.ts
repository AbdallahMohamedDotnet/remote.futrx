import { pushServiceWorkerApi } from "../../api/pushServiceWorkerApi";

type ChatOpener = (chatId: string | null) => void;

class PushNotificationState {
  #visibleChatId: string | null = null;
  #openChat: ChatOpener | null = null;

  /** Registers for push and routes notification taps into chat selection. */
  connect(openChat: ChatOpener): void {
    // Keep registration first: it installs the listener before asking the
    // browser to update the worker, matching the page's startup sequence.
    void pushServiceWorkerApi.register();
    this.#openChat = openChat;
    pushServiceWorkerApi.connect({
      visibleChatId: () => this.#chatInFocus(),
      openChat: (chatId) => this.#openChat?.(chatId),
    });
  }

  /** Reports which chat is on screen, so the worker can suppress its notification. */
  setVisibleChat(chatId: string | null): void {
    this.#visibleChatId = chatId;
  }

  #chatInFocus(): string | null {
    // Only claim a chat when this window is genuinely in front; a background
    // tab showing the chat should still raise a notification.
    return document.visibilityState === "visible" && document.hasFocus()
      ? this.#visibleChatId
      : null;
  }
}

export const pushNotificationState = new PushNotificationState();
