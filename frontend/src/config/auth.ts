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

/** How often the setup screen re-checks whether an admin has been configured. */
export const ADMIN_SETUP_POLL_INTERVAL_MS = 3_000;
