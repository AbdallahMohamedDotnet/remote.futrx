import { requestJson } from "./apiRequest";
import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/webSocketUrl";
import type { CodexAuthStatus, CodexDeviceLogin } from "../models/auth";

export const codexAuthApi = {
  status: () => requestJson<CodexAuthStatus>("GET", "/api/codex/auth-status"),
  startDeviceLogin: () => requestJson<CodexDeviceLogin>("POST", "/api/codex/login/device", {}),
  subscribe: (onStatus: (status: CodexAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl("/ws/codex/auth-status"),
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
