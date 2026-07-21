import type { AuthSession } from "../models/auth";

export const UNAUTHENTICATED_SESSION: AuthSession = {
  noAuth: false,
  authenticated: false,
  claimed: false,
  adminEmail: "",
  email: "",
  isAdmin: false,
  isRegistered: false,
};
