import { requestJson } from "../apiRequest";
import type { DirListing, FileSearchResult } from "../../models/files";
import { API_ROUTES } from "../../config/routes";

export const chatFilesApi = {
  listDir: (id: string, path = "") =>
    requestJson<DirListing>("GET", API_ROUTES.chats.files(id, path)),

  searchFiles: (id: string, query: string) =>
    requestJson<FileSearchResult>("GET", API_ROUTES.chats.filesSearch(id, query)),

  fileDownloadUrl: (id: string, path: string) =>
    API_ROUTES.chats.fileDownload(id, path),

  folderDownloadUrl: (id: string, path = "") =>
    API_ROUTES.chats.folderDownload(id, path),
};
