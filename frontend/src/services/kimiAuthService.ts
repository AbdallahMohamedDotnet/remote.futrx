import { json } from "../api/http";
import type { KimiAuthStatus, KimiDeviceLogin } from "../models/auth";

export const kimiAuthService = {
  status: () => json<KimiAuthStatus>("GET", "/api/kimi/auth-status"),
  startDeviceLogin: () => json<KimiDeviceLogin>("POST", "/api/kimi/login/device", {}),
};
