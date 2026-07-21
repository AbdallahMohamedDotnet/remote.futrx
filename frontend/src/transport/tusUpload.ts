import * as tus from "tus-js-client";

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

/**
 * Start a resumable upload. Connection drops are retried automatically and
 * fingerprinted upload URLs are retained so later browser sessions can resume.
 */
export function startResumableUpload(
  file: File,
  options: ResumableUploadOptions,
): ResumableUploadHandle {
  const upload = new tus.Upload(file, {
    endpoint: options.endpoint,
    // 5 MiB chunks: small enough for snappy progress + retry, large enough
    // that HTTP/TLS overhead stays well under 1%.
    chunkSize: 5 * 1024 * 1024,
    retryDelays: [0, 1000, 3000, 5000, 10000, 20000],
    // tus-js-client stores the upload URL in localStorage under this identity.
    fingerprint: options.fingerprint,
    storeFingerprintForResuming: true,
    removeFingerprintOnSuccess: true,
    metadata: options.metadata,
    onError(error) {
      options.onError(error);
    },
    onProgress(loaded, total) {
      options.onProgress(loaded, total);
    },
    onSuccess() {
      options.onSuccess();
    },
  });

  // If a previous session left a partial upload, resume it; otherwise start fresh.
  void upload.findPreviousUploads().then((previousUploads) => {
    if (previousUploads.length > 0) {
      upload.resumeFromPreviousUpload(previousUploads[0]);
    }
    upload.start();
  });

  return {
    abort: () => upload.abort(true),
  };
}
