import { requestJson } from "./apiRequest";
import { subscribeToJsonMessages } from "../transport/jsonMessageSubscription";
import type { ClaudeAuthStatus, ClaudeLoginStart } from "../models/auth";
import { API_ROUTES, WEB_SOCKET_ROUTES } from "../config/routes";

export const claudeAuthApi = {
  fetchStatus: () =>
    requestJson<ClaudeAuthStatus>("GET", API_ROUTES.claudeAuth.status),
  startLogin: () =>
    requestJson<ClaudeLoginStart>("POST", API_ROUTES.claudeAuth.startLogin, {}),
  submitCode: (code: string) =>
    requestJson<{ success: boolean }>("POST", API_ROUTES.claudeAuth.submitCode, { code }),
  cancelLogin: () =>
    requestJson<{ ok: boolean }>("POST", API_ROUTES.claudeAuth.cancelLogin, {}),
  subscribe: (onStatus: (status: ClaudeAuthStatus) => void) =>
    subscribeToJsonMessages(WEB_SOCKET_ROUTES.claudeAuthStatus, onStatus),
};
