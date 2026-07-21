import { requestJson } from "./apiRequest";
import { subscribeToJsonMessages } from "../transport/jsonMessageSubscription";
import type { CodexAuthStatus, CodexDeviceLogin } from "../models/auth";
import { API_ROUTES, WEB_SOCKET_ROUTES } from "../config/routes";

export const codexAuthApi = {
  status: () => requestJson<CodexAuthStatus>("GET", API_ROUTES.codexAuth.status),
  startDeviceLogin: () =>
    requestJson<CodexDeviceLogin>(
      "POST",
      API_ROUTES.codexAuth.startDeviceLogin,
      {}
    ),
  subscribe: (onStatus: (status: CodexAuthStatus) => void) =>
    subscribeToJsonMessages(WEB_SOCKET_ROUTES.codexAuthStatus, onStatus),
};
