export interface AuthSession {
  noAuth: boolean;
  authenticated: boolean;
  claimed: boolean;
  adminEmail: string;
  email: string;
  isAdmin: boolean;
}

export interface ClaudeAuthStatus {
  authenticated: boolean;
}

export interface ClaudeLoginStart {
  url: string;
  resumed?: boolean;
}

export type ClaudeLoginPhase =
  | "idle"
  | "starting"
  | "awaiting-code"
  | "submitting"
  | "done"
  | "error";
