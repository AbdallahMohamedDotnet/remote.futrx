import { json } from "../transport/http";
import { ReconnectingJsonWebSocket } from "../transport/ReconnectingJsonWebSocket";
import { kimiAuthWebSocketUrl } from "../transport/websocket";
import type { KimiAuthStatus, KimiDeviceLogin } from "../models/auth";

export const kimiAuthApi = {
  status: () => json<KimiAuthStatus>("GET", "/api/kimi/auth-status"),
  startDeviceLogin: () => json<KimiDeviceLogin>("POST", "/api/kimi/login/device", {}),
  subscribe: (onStatus: (status: KimiAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      url: kimiAuthWebSocketUrl,
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
