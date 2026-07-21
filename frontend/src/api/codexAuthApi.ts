import { json } from "../transport/http";
import { ReconnectingJsonWebSocket } from "../transport/ReconnectingJsonWebSocket";
import { webSocketUrl } from "../transport/websocket";
import type { CodexAuthStatus, CodexDeviceLogin } from "../models/auth";

export const codexAuthApi = {
  status: () => json<CodexAuthStatus>("GET", "/api/codex/auth-status"),
  startDeviceLogin: () => json<CodexDeviceLogin>("POST", "/api/codex/login/device", {}),
  subscribe: (onStatus: (status: CodexAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      url: () => webSocketUrl("/ws/codex/auth-status"),
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
