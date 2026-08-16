import { requestJson } from "./apiRequest";
import type {
  SecurityPreferencesInput,
  SecuritySettings,
  TwoFactorEnrollment,
} from "../models/security";
import { API_ROUTES } from "../config/routes";

export const securityApi = {
  fetch: () => requestJson<SecuritySettings>("GET", API_ROUTES.security.summary),
  beginEnrollment: () =>
    requestJson<TwoFactorEnrollment>("POST", API_ROUTES.security.enroll),
  confirmEnrollment: (enrollmentToken: string, code: string) =>
    requestJson<{ recoveryCodes: string[] }>("POST", API_ROUTES.security.confirm, {
      enrollmentToken,
      code,
    }),
  disable: (code: string) =>
    requestJson<void>("POST", API_ROUTES.security.disable, { code }),
  regenerateRecoveryCodes: (code: string) =>
    requestJson<{ recoveryCodes: string[] }>(
      "POST",
      API_ROUTES.security.regenerateRecoveryCodes,
      { code }
    ),
  setPreferences: (input: SecurityPreferencesInput) =>
    requestJson<SecuritySettings>("POST", API_ROUTES.security.preferences, input),
  ackAlert: () => requestJson<void>("POST", API_ROUTES.security.ackAlert),
};
