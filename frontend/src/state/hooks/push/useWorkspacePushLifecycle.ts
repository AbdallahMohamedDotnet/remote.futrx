import { useEffect } from "preact/hooks";

import { pushSubscriptionApi } from "../../../api/pushSubscriptionApi";
import type { WorkspaceView } from "../../logic/workspace/workspaceUiState";
import { pushNotificationStore } from "../../stores/pushNotificationStore";
import { pushPresenceStore } from "../../stores/pushPresenceStore";

interface WorkspacePushLifecycleOptions {
  activeChatId: string | null;
  view: WorkspaceView;
  openChat: (chatId: string) => void;
}

/** Keeps subscription ownership, worker routing, and presence in sync. */
export function useWorkspacePushLifecycle({
  activeChatId,
  view,
  openChat,
}: WorkspacePushLifecycleOptions): void {
  // Register the worker on every boot so a deployed sw.js replaces the
  // installed one, and route notification taps into chat selection.
  useEffect(() => {
    void pushSubscriptionApi.reconcileCurrentAccount();
    pushNotificationStore.connect((chatId) => {
      if (chatId) openChat(chatId);
    });
  }, [openChat]);

  // Say which chat is on screen, so nothing interrupts the user about the one
  // they are already watching. The worker covers this browser; the server
  // covers the user's other devices, which the worker cannot see.
  useEffect(() => {
    const onScreen = view === "chat" ? activeChatId : null;
    pushNotificationStore.setVisibleChat(onScreen);
    pushPresenceStore.setWatchedChat(onScreen);
  }, [activeChatId, view]);
}
