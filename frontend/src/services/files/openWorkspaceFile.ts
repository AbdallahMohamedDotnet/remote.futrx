import type { FileOpenRequest } from "../../models/files.ts";
import { API_ROUTES } from "../../config/routes.ts";
import { fileService } from "./fileService.ts";
import { resolveFileOpener } from "./resolveFileOpener.ts";
import { fileOpenerStore } from "../../state/stores/files/fileOpenerStore.ts";
import { mediaViewerStore } from "../../state/stores/media/mediaViewerStore.ts";

/** Shared file opening for application surfaces, chat links and media viewers. */
export function openWorkspaceFile(
  request: FileOpenRequest & { chatId: string },
): void {
  const relative = workspaceRelativePath(request);
  if (!relative) return;
  const name = relative.split("/").at(-1) || relative;
  const target = fileService.openAction(name);
  if (target.action === "media") {
    mediaViewerStore.getState().open({
      url: API_ROUTES.chats.mediaOpen(request.chatId, request.path),
      name,
      kind: target.kind,
    });
    return;
  }
  const url = workspaceFileEditorUrl(request);
  if (url) {
    window.open(url, "_blank", "noopener");
    return;
  }
  window.location.assign(
    API_ROUTES.chats.fileDownload(request.chatId, relative),
  );
}

/** The editor URL `openWorkspaceFile` would open in a new tab for this file,
 *  or null when it would show or download it instead. Lets a surface that
 *  must open the tab itself (an iframe, inside its own click) do so. */
export function workspaceFileEditorUrl(
  request: FileOpenRequest & { chatId: string },
): string | null {
  const relative = workspaceRelativePath(request);
  if (!relative) return null;
  const name = relative.split("/").at(-1) || relative;
  if (fileService.openAction(name).action !== "application") return null;
  return resolveFileOpener(
    fileOpenerStore.getState().forProject(request.projectId),
    request,
  );
}

// The path below /workspace, or null for anything outside it.
function workspaceRelativePath(
  request: FileOpenRequest & { chatId: string },
): string | null {
  if (!request.chatId || !request.path.startsWith("/workspace/")) return null;
  const relative = request.path.slice("/workspace/".length);
  if (
    !relative ||
    relative.split("/").some((part) => part === ".." || part === ".")
  )
    return null;
  return relative;
}
