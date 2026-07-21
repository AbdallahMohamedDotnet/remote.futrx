import type { AuthSession } from "../models/auth";

export const UNAUTHENTICATED_SESSION: AuthSession = {
  authenticated: false,
  claimed: false,
  localAdminConfigured: false,
  googleOAuthEnabled: false,
  googleClientId: "",
  adminEmail: "",
  email: "",
  isAdmin: false,
  isRegistered: false,
};
