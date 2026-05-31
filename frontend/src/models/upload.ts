export interface Attachment {
  id: string;
  name: string;
  size: number;
  serverPath: string;
  isImage: boolean;
  objectUrl?: string;
  /** 0–1, only set while uploading. */
  progress?: number;
  /** Error message from the tus client; presence implies the upload failed. */
  error?: string;
}

// Kept around for backwards compatibility — no longer returned by the new
// resumable endpoint, but a few components still type-check against it.
export interface UploadResult {
  name: string;
  path?: string;
  size?: number;
  error?: string;
}

export interface UploadChatFilesResponse {
  cwd: string;
  results: UploadResult[];
}
