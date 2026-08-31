import { sendHttpRequest } from "../transport/http";
import { requestJson } from "./apiRequest";
import type { AuthSession, GoogleOAuthSettings } from "../models/auth";
import { API_ROUTES } from "../config/routes";
import { UNAUTHENTICATED_SESSION } from "../config/auth";

export async function fetchAuthSession(): Promise<AuthSession> {
  try {
    const response = await sendHttpRequest("GET", API_ROUTES.authSession);
    if (!response.ok) return UNAUTHENTICATED_SESSION;
    const data = await response.json();
    return {
      authenticated: !!data.authenticated,
      claimed: !!data.claimed,
      localAdminConfigured: !!data.localAdminConfigured,
      googleOAuthEnabled: !!data.googleOAuthEnabled,
      googleClientId: data.googleClientId ?? "",
      adminEmail: data.adminEmail ?? "",
      email: data.email ?? "",
      isAdmin: !!data.isAdmin,
      isRegistered: !!data.isRegistered,
    };
  } catch {
    return UNAUTHENTICATED_SESSION;
  }
}

export const localAuthApi = {
  // setupToken is sent in the body, never the query string, so it stays out
  // of reverse-proxy access logs.
  claim: (email: string, password: string, setupToken: string) =>
    requestLocalAuth("/auth/local/claim", { email, password, setupToken }),
  login: (email: string, password: string) =>
    requestLocalAuth("/auth/local/login", { email, password }),
};

export const googleOAuthApi = {
  get: () =>
    requestJson<GoogleOAuthSettings>("GET", API_ROUTES.googleOAuth),
  save: (clientId: string, clientSecret: string) =>
    requestJson<GoogleOAuthSettings>("PUT", API_ROUTES.googleOAuth, {
      clientId,
      clientSecret,
    }),
};

async function requestLocalAuth(url: string, body: Record<string, string>): Promise<AuthSession> {
  const response = await sendHttpRequest("POST", url, body);
  if (!response.ok) {
    let message = `${response.status}`;
    try {
      message = (await response.json()).error || message;
    } catch {}
    throw new Error(message);
  }
  return response.json() as Promise<AuthSession>;
}
