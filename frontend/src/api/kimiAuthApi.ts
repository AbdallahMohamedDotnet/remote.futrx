import { requestJson } from "./apiRequest";
import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/webSocketUrl";
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
