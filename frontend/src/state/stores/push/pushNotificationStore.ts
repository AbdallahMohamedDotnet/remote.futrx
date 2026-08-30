import { createStore } from "zustand/vanilla";
import { pushServiceWorkerApi } from "../../../api/pushServiceWorkerApi";
import { isPushPageFocused } from "./pushPageFocus";

type ChatOpener = (chatId: string | null) => void;

interface PushNotificationState {
  visibleChatId: string | null;
  connect: (openChat: ChatOpener) => void;
  setVisibleChat: (chatId: string | null) => void;
}

export const pushNotificationStore = createStore<PushNotificationState>()((set, get) => ({
  visibleChatId: null,
  /** Registers for push and routes notification taps into chat selection. */
  connect: (openChat) => {
    // Keep registration first: it installs the listener before asking the
    // browser to update the worker, matching the page's startup sequence.
    void pushServiceWorkerApi.register();
    pushServiceWorkerApi.connect({
      visibleChatId: () => {
        // A background tab showing the chat should still raise a notification.
        return isPushPageFocused() ? get().visibleChatId : null;
      },
      openChat,
    });
  },

  /** Reports which chat is on screen, so the worker can suppress its notification. */
  setVisibleChat: (visibleChatId) => set({ visibleChatId }),
}));
