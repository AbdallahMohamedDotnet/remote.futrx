import * as tus from "tus-js-client";

export interface UploadHandle {
  /** Tell the server we no longer want this upload; deletes the partial file on disk. */
  abort: () => Promise<void>;
}

export interface ChatUploadCallbacks {
  onProgress: (loaded: number, total: number) => void;
  onSuccess: () => void;
  onError: (err: Error) => void;
}

/**
 * Start a resumable upload for one file via the tus protocol. Survives
 * connection drops automatically; survives browser tab close via the
 * fingerprint-keyed URL store in localStorage.
 */
export function startChatUpload(
  chatId: string,
  file: File,
  cb: ChatUploadCallbacks
): UploadHandle {
  const upload = new tus.Upload(file, {
    endpoint: "/api/uploads",
    // 5 MiB chunks: small enough for snappy progress + retry, large enough
    // that HTTP/TLS overhead stays well under 1%.
    chunkSize: 5 * 1024 * 1024,
    retryDelays: [0, 1000, 3000, 5000, 10000, 20000],
    // Resume identity. tus-js-client stores the upload URL in localStorage
    // keyed by this fingerprint so a tab refresh / browser restart can
    // continue from the last server-acknowledged byte.
    fingerprint: async () =>
      `chat:${chatId}:${file.name}:${file.size}:${file.lastModified}`,
    storeFingerprintForResuming: true,
    removeFingerprintOnSuccess: true,
    metadata: {
      chatId,
      filename: file.name,
      filetype: file.type || "application/octet-stream",
    },
    onError(err) {
      cb.onError(err);
    },
    onProgress(loaded, total) {
      cb.onProgress(loaded, total);
    },
    onSuccess() {
      cb.onSuccess();
    },
  });

  // If a previous session left a partial upload, resume it; otherwise start fresh.
  void upload.findPreviousUploads().then((prev) => {
    if (prev.length > 0) upload.resumeFromPreviousUpload(prev[0]);
    upload.start();
  });

  return {
    abort: () => upload.abort(true),
  };
}
