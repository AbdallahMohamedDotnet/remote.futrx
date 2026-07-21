import { requestJson } from "../transport/http";
import { ReconnectingJsonWebSocket } from "../transport/reconnectingJsonSocket";
import { webSocketUrl } from "../transport/websocket";
import type { ClaudeAuthStatus, ClaudeLoginStart } from "../models/auth";

export const claudeAuthApi = {
  status: () => requestJson<ClaudeAuthStatus>("GET", "/api/claude/auth-status"),
  startLogin: () => requestJson<ClaudeLoginStart>("POST", "/api/claude/login/start", {}),
  submitCode: (code: string) =>
    requestJson<{ success: boolean }>("POST", "/api/claude/login/code", { code }),
  cancelLogin: () => requestJson<{ ok: boolean }>("POST", "/api/claude/login/cancel", {}),
  subscribe: (onStatus: (status: ClaudeAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      resolveUrl: () => webSocketUrl("/ws/claude/auth-status"),
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
