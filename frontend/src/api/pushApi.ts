import { requestJson } from "./apiRequest";
import { API_ROUTES } from "../config/routes";
import type { PushConfig, PushSubscriptionPayload } from "../models/push";

export const pushApi = {
  config: () => requestJson<PushConfig>("GET", API_ROUTES.push.config),
  subscribe: (subscription: PushSubscriptionPayload) =>
    requestJson<void>("POST", API_ROUTES.push.subscriptions, subscription),
  unsubscribe: (endpoint: string) =>
    requestJson<void>("DELETE", API_ROUTES.push.subscriptions, { endpoint }),
  test: () => requestJson<void>("POST", API_ROUTES.push.test),
};
