import { requestJson } from "./apiRequest";
import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/webSocketUrl";
import type { CodexAuthStatus, CodexDeviceLogin } from "../models/auth";
import { API_ROUTES, WEB_SOCKET_ROUTES } from "../config/routes";

export const codexAuthApi = {
  status: () => requestJson<CodexAuthStatus>("GET", API_ROUTES.codexAuth.status),
  startDeviceLogin: () =>
    requestJson<CodexDeviceLogin>(
      "POST",
      API_ROUTES.codexAuth.startDeviceLogin,
      {}
    ),
  subscribe: (onStatus: (status: CodexAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl(WEB_SOCKET_ROUTES.codexAuthStatus),
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
