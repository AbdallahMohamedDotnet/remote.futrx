export interface UploadHandle {
  /** Tell the server we no longer want this upload; deletes the partial file on disk. */
  abort: () => Promise<void>;
}

export interface ChatUploadCallbacks {
  onProgress: (loaded: number, total: number) => void;
  onSuccess: () => void;
  onError: (error: Error) => void;
}
