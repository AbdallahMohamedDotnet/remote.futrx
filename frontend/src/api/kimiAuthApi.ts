import { json } from "../transport/http";
import type { KimiAuthStatus, KimiDeviceLogin } from "../models/auth";

export const kimiAuthApi = {
  status: () => json<KimiAuthStatus>("GET", "/api/kimi/auth-status"),
  startDeviceLogin: () => json<KimiDeviceLogin>("POST", "/api/kimi/login/device", {}),
};
