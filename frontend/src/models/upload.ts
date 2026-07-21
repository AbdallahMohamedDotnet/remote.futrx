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
