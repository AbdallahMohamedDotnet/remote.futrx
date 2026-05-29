import { request } from "../api/http";
import type { UploadChatFilesResponse } from "../models/upload";

export async function uploadChatFiles(
  chatId: string,
  files: File[]
): Promise<UploadChatFilesResponse> {
  const form = new FormData();
  for (const file of files) form.append("files", file, file.name);

  const response = await request("POST", `/api/chats/${encodeURIComponent(chatId)}/upload`, form);
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `upload failed: ${response.status}`);
  return data;
}
