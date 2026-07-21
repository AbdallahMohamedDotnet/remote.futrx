import { startResumableUpload } from "../transport/tusUpload";

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
  return startResumableUpload(file, {
    endpoint: "/api/uploads",
    // Resume identity. tus-js-client stores the upload URL in localStorage
    // keyed by this fingerprint so a tab refresh / browser restart can
    // continue from the last server-acknowledged byte.
    fingerprint: async () =>
      `chat:${chatId}:${file.name}:${file.size}:${file.lastModified}`,
    metadata: {
      chatId,
      filename: file.name,
      filetype: file.type || "application/octet-stream",
    },
    onError(error) {
      cb.onError(error);
    },
    onProgress(loaded, total) {
      cb.onProgress(loaded, total);
    },
    onSuccess() {
      cb.onSuccess();
    },
  });
}
