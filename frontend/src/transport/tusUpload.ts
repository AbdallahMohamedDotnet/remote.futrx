import * as tus from "tus-js-client";
import type {
  ResumableUploadHandle,
  ResumableUploadOptions,
} from "../types/transport";

/**
 * Start a resumable upload. Connection drops are retried automatically and
 * fingerprinted upload URLs are retained so later browser sessions can resume.
 */
export function startResumableUpload(
  file: File,
  options: ResumableUploadOptions,
): ResumableUploadHandle {
  return new TusResumableUpload(file, options);
}

class TusResumableUpload implements ResumableUploadHandle {
  readonly #upload: tus.Upload;

  constructor(file: File, options: ResumableUploadOptions) {
    this.#upload = new tus.Upload(file, {
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

    this.#resumeOrStart();
  }

  readonly abort = (): Promise<void> => this.#upload.abort(true);

  #resumeOrStart(): void {
    // If a previous session left a partial upload, resume it; otherwise start fresh.
    void this.#upload.findPreviousUploads().then((previousUploads) => {
      if (previousUploads.length > 0) {
        this.#upload.resumeFromPreviousUpload(previousUploads[0]);
      }
      this.#upload.start();
    });
  }
}
