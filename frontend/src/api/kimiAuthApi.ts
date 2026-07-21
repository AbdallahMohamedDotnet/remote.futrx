import { requestJson } from "./apiRequest";
import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/webSocketUrl";
import type { KimiAuthStatus, KimiDeviceLogin } from "../models/auth";
import { API_ROUTES, WEB_SOCKET_ROUTES } from "../config/routes";

export const kimiAuthApi = {
  status: () => requestJson<KimiAuthStatus>("GET", API_ROUTES.kimiAuth.status),
  startDeviceLogin: () =>
    requestJson<KimiDeviceLogin>(
      "POST",
      API_ROUTES.kimiAuth.startDeviceLogin,
      {}
    ),
  subscribe: (onStatus: (status: KimiAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl(WEB_SOCKET_ROUTES.kimiAuthStatus),
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
