import { requestJson } from "./apiRequest";
import { subscribeToJsonMessages } from "../transport/jsonMessageSubscription";
import type { KimiAuthStatus, KimiDeviceLogin } from "../models/auth";
import { API_ROUTES, WEB_SOCKET_ROUTES } from "../config/routes";

export const kimiAuthApi = {
  status: () => requestJson<KimiAuthStatus>("GET", API_ROUTES.kimiAuth.status),
  startDeviceLogin: () =>
    requestJson<KimiDeviceLogin>(
      "POST",
      API_ROUTES.kimiAuth.startDeviceLogin,
      {}
    ),
  subscribe: (onStatus: (status: KimiAuthStatus) => void) =>
    subscribeToJsonMessages(WEB_SOCKET_ROUTES.kimiAuthStatus, onStatus),
};
