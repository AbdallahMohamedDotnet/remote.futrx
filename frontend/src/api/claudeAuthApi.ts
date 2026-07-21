import { json } from "../transport/http";
import { ReconnectingJsonWebSocket } from "../transport/ReconnectingJsonWebSocket";
import { claudeAuthWebSocketUrl } from "../transport/websocket";
import type { ClaudeAuthStatus, ClaudeLoginStart } from "../models/auth";

export const claudeAuthApi = {
  status: () => json<ClaudeAuthStatus>("GET", "/api/claude/auth-status"),
  startLogin: () => json<ClaudeLoginStart>("POST", "/api/claude/login/start", {}),
  submitCode: (code: string) =>
    json<{ success: boolean }>("POST", "/api/claude/login/code", { code }),
  cancelLogin: () => json<{ ok: boolean }>("POST", "/api/claude/login/cancel", {}),
  subscribe: (onStatus: (status: ClaudeAuthStatus) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      url: claudeAuthWebSocketUrl,
      onMessage: onStatus,
    });
    connection.start();
    return () => connection.stop();
  },
};
