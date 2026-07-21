import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";

export function attachmentBasePathForChat(chat: ChatMeta, projects: ProjectMeta[]) {
  // Uploads are stored in a fixed .uploads/ directory at the workspace root,
  // matching the server (chat.UploadTarget). Anchoring at the root rather than
  // the chat's cwd keeps the path stable and exactly predictable here, so the
  // prompt path always matches where the server actually wrote the file.
  const project = chat.projectId ? projects.find((item) => item.id === chat.projectId) : undefined;
  if (project) return "/workspace/.uploads";

  const cwd = normalizePath(chat.cwd || "");
  return cwd ? `${cwd}/.uploads` : "/.uploads";
}

function normalizePath(path: string) {
  const trimmed = path.trim();
  if (!trimmed) return "";
  return trimmed.replace(/\/+$/, "") || "/";
}
