import { json } from "../api/http";

// Mirrors the backend response shape from
// POST /api/projects/{id}/login-sessions and GET /.../{sid}.
export interface LoginSessionStart {
  id: string;
  wsPath: string;
  url: string;
  name: string;
  secretName: string;
  expiresAt: number;
}

export interface CaptureResult {
  secretName: string;
  sizeBytes: number;
  cookieCount: number;
  originCount: number;
}

export const loginSessionService = {
  start: (projectId: string, url: string, name: string) =>
    json<LoginSessionStart>(
      "POST",
      `/api/projects/${encodeURIComponent(projectId)}/login-sessions`,
      { url, name }
    ),

  stop: (projectId: string, sessionId: string) =>
    json<{ ok: boolean }>(
      "DELETE",
      `/api/projects/${encodeURIComponent(projectId)}/login-sessions/${encodeURIComponent(sessionId)}`
    ),

  capture: (projectId: string, sessionId: string) =>
    json<CaptureResult>(
      "POST",
      `/api/projects/${encodeURIComponent(projectId)}/login-sessions/${encodeURIComponent(sessionId)}/capture`,
      {}
    ),
};
