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
  if (!request.chatId || !request.path.startsWith("/workspace/")) return;
  const relative = request.path.slice("/workspace/".length);
  if (
    !relative ||
    relative.split("/").some((part) => part === ".." || part === ".")
  )
    return;
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
  if (target.action === "application") {
    const url = resolveFileOpener(
      fileOpenerStore.getState().forProject(request.projectId),
      request,
    );
    if (url) {
      window.open(url, "_blank", "noopener");
      return;
    }
  }
  window.location.assign(
    API_ROUTES.chats.fileDownload(request.chatId, relative),
  );
}
