import { json } from "../api/http";
import type { CodexAuthStatus } from "../models/auth";

export const codexAuthService = {
  status: () => json<CodexAuthStatus>("GET", "/api/codex/auth-status"),
  loginWithAPIKey: (apiKey: string) =>
    json<{ success: boolean }>("POST", "/api/codex/login/api-key", { apiKey }),
};
