import { sendHttpRequest } from "../transport/http";
import type { AuthSession } from "../models/auth";
import { API_ROUTES } from "../config/routes";
import { API_RESPONSE_STATUS } from "../config/api";
import { UNAUTHENTICATED_SESSION } from "../config/auth";

export async function fetchAuthSession(): Promise<AuthSession> {
  try {
    const response = await sendHttpRequest("GET", API_ROUTES.authSession);
    if (response.status === API_RESPONSE_STATUS.notFound) {
      return {
        ...UNAUTHENTICATED_SESSION,
        noAuth: true,
        authenticated: true,
        isAdmin: true,
        isRegistered: true,
      };
    }
    if (!response.ok) return UNAUTHENTICATED_SESSION;
    const data = await response.json();
    return {
      noAuth: false,
      authenticated: !!data.authenticated,
      claimed: !!data.claimed,
      adminEmail: data.adminEmail ?? "",
      email: data.email ?? "",
      isAdmin: !!data.isAdmin,
      isRegistered: !!data.isRegistered,
    };
  } catch {
    return UNAUTHENTICATED_SESSION;
  }
}
