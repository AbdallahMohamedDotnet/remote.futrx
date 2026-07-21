import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/webSocketUrl";
import type { WorkspaceMessage } from "../types/workspaceApi";

export const workspaceApi = {
  subscribe: (onMessage: (message: WorkspaceMessage) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl("/ws/workspace"),
      onMessage,
    });
    connection.start();
    return () => connection.stop();
  },
};
