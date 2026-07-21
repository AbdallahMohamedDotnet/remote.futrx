import { json } from "../transport/http";
import type { ClaudeAuthStatus, ClaudeLoginStart } from "../models/auth";

export const claudeAuthService = {
  status: () => json<ClaudeAuthStatus>("GET", "/api/claude/auth-status"),
  startLogin: () => json<ClaudeLoginStart>("POST", "/api/claude/login/start", {}),
  submitCode: (code: string) =>
    json<{ success: boolean }>("POST", "/api/claude/login/code", { code }),
  cancelLogin: () => json<{ ok: boolean }>("POST", "/api/claude/login/cancel", {}),
};
