import { json } from "../api/http";
import type { CodexAuthStatus, CodexDeviceLogin } from "../models/auth";

export const codexAuthService = {
  status: () => json<CodexAuthStatus>("GET", "/api/codex/auth-status"),
  startDeviceLogin: () => json<CodexDeviceLogin>("POST", "/api/codex/login/device", {}),
};
