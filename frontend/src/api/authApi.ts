import { sendHttpRequest } from "../transport/http";
import type { AuthSession } from "../models/auth";

const unauthenticated: AuthSession = {
  noAuth: false,
  authenticated: false,
  claimed: false,
  adminEmail: "",
  email: "",
  isAdmin: false,
  isRegistered: false,
};

export async function getAuthSession(): Promise<AuthSession> {
  try {
    const response = await sendHttpRequest("GET", "/auth/me");
    if (response.status === 404) {
      return {
        ...unauthenticated,
        noAuth: true,
        authenticated: true,
        isAdmin: true,
        isRegistered: true,
      };
    }
    if (!response.ok) return unauthenticated;
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
    return unauthenticated;
  }
}
