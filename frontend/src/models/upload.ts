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

export interface Attachment {
  id: string;
  name: string;
  size: number;
  serverPath: string;
  isImage: boolean;
  objectUrl?: string;
}
