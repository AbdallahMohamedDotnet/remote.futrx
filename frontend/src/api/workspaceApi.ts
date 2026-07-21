import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/webSocketUrl";
import type { WorkspaceMessage } from "../types/workspaceApi";
import { WEB_SOCKET_ROUTES } from "../config/routes";

export const workspaceApi = {
  subscribe: (onMessage: (message: WorkspaceMessage) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl(WEB_SOCKET_ROUTES.workspace),
      onMessage,
    });
    connection.start();
    return () => connection.stop();
  },
};
