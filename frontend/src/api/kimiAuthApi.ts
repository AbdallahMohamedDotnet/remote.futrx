import { requestJson } from "../transport/http";
import { ReconnectingJsonWebSocket } from "../transport/ReconnectingJsonWebSocket";
import { webSocketUrl } from "../transport/websocket";
import type { KimiAuthStatus, KimiDeviceLogin } from "../models/auth";

export const kimiAuthApi = {
  status: () => requestJson<KimiAuthStatus>("GET", "/api/kimi/auth-status"),
  startDeviceLogin: () => requestJson<KimiDeviceLogin>("POST", "/api/kimi/login/device", {}),
  subscribe: (onStatus: (status: KimiAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl("/ws/kimi/auth-status"),
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
