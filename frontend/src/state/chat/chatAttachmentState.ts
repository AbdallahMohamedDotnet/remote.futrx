import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";
import type { Attachment } from "../../models/upload";

class ChatAttachmentState {
  basePath(chat: ChatMeta, projects: ProjectMeta[]): string {
    const project = chat.projectId
      ? projects.find((candidate) => candidate.id === chat.projectId)
      : undefined;
    if (project) return "/workspace/.uploads";

    const cwd = this.normalizePath(chat.cwd || "");
    return cwd ? `${cwd}/.uploads` : "/.uploads";
  }

  uniqueUploadName(name: string, token: string): string {
    const cleaned = name.split(/[\\/]/).pop()?.trim() || name.trim();
    if (!cleaned) return `file-${token}`;
    const dot = cleaned.lastIndexOf(".");
    if (dot <= 0) return `${cleaned}-${token}`;
    return `${cleaned.slice(0, dot)}-${token}${cleaned.slice(dot)}`;
  }

  absoluteUploadPath(basePath: string, fileName: string): string {
    const safeName = fileName.split(/[\\/]/).pop()?.trim() || fileName.trim();
    if (!safeName) return "";
    if (safeName.startsWith("/")) return safeName;

    const base = basePath.trim().replace(/\/+$/, "");
    if (!base) return safeName;
    return `${base}/${safeName}`;
  }

  revoke(attachment: Attachment): void {
    if (attachment.objectUrl) URL.revokeObjectURL(attachment.objectUrl);
  }

  private normalizePath(path: string): string {
    const trimmed = path.trim();
    if (!trimmed) return "";
    return trimmed.replace(/\/+$/, "") || "/";
  }
}

export const chatAttachmentState = new ChatAttachmentState();
