export interface AuthSession {
  noAuth: boolean;
  authenticated: boolean;
  claimed: boolean;
  adminEmail: string;
  email: string;
  isAdmin: boolean;
  isRegistered: boolean;
}

export interface ClaudeAuthStatus {
  authenticated: boolean;
}

export interface ClaudeLoginStart {
  url: string;
  resumed?: boolean;
}

export interface CodexAuthStatus {
  authenticated: boolean;
  authMode?: string;
  usesApiKey?: boolean;
  deviceLogin?: CodexDeviceLogin;
}

export interface CodexDeviceLogin {
  active: boolean;
  verificationUri?: string;
  userCode?: string;
  startedAt?: number;
  expiresAt?: number;
  completed?: boolean;
  error?: string;
}

export type ClaudeLoginPhase =
  | "idle"
  | "starting"
  | "awaiting-code"
  | "submitting"
  | "done"
  | "error";
