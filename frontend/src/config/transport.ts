export const HTTP_JSON_CONTENT_TYPE = "application/json";
export const DEFAULT_HTTP_CREDENTIALS = "same-origin";

export const WEB_SOCKET_PROTOCOLS = {
  secure: "wss:",
  insecure: "ws:",
} as const;

export const JSON_SOCKET_RECONNECT_POLICY = {
  initialDelayMs: 400,
  maxDelayMs: 5_000,
} as const;

export const RESUMABLE_UPLOAD_CONFIG = {
  chunkSizeBytes: 5 * 1024 * 1024,
  retryDelaysMs: [0, 1_000, 3_000, 5_000, 10_000, 20_000],
  storeFingerprintForResuming: true,
  removeFingerprintOnSuccess: true,
} as const;
