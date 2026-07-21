export type ApplicationPath = `/${string}`;

export type HttpMethod = "DELETE" | "GET" | "PATCH" | "POST" | "PUT";

export interface ReconnectingJsonWebSocketOptions<TMessage> {
  resolveUrl: () => string;
  onMessage: (message: TMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
}

export interface ResumableUploadHandle {
  abort: () => Promise<void>;
}

export interface ResumableUploadOptions {
  endpoint: string;
  fingerprint: () => Promise<string>;
  metadata: Record<string, string>;
  onProgress: (loaded: number, total: number) => void;
  onSuccess: () => void;
  onError: (error: Error) => void;
}

export interface WebSocketConnectionOptions {
  url: string;
  binaryType?: BinaryType;
  onOpen: () => void;
  onMessage: (data: unknown) => void;
  onError: () => void;
  onClose: () => void;
}
