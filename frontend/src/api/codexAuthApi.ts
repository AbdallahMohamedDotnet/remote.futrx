import { json } from "../transport/http";
import type { CodexAuthStatus, CodexDeviceLogin } from "../models/auth";

export const codexAuthApi = {
  status: () => json<CodexAuthStatus>("GET", "/api/codex/auth-status"),
  startDeviceLogin: () => json<CodexDeviceLogin>("POST", "/api/codex/login/device", {}),
};
