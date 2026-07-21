import { requestJson } from "../apiRequest";
import type { FileTreeResponse } from "../../models/files";
import { API_ROUTES } from "../../config/routes";

export const chatFilesApi = {
  files: (id: string) =>
    requestJson<FileTreeResponse>("GET", API_ROUTES.chats.files(id)),

  fileDownloadUrl: (id: string, dir: string, path: string) =>
    API_ROUTES.chats.fileDownload(id, dir, path),

  folderDownloadUrl: (id: string, dir: string, path = "") =>
    API_ROUTES.chats.folderDownload(id, dir, path),
};
