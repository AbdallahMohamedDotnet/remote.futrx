import assert from "node:assert/strict";
import test from "node:test";
import { openWorkspaceFile } from "./openWorkspaceFile.ts";
import { fileOpenerStore } from "../../state/stores/files/fileOpenerStore.ts";
import { mediaViewerStore } from "../../state/stores/media/mediaViewerStore.ts";

test("application files use scoped openers, media previews and download fallback", () => {
  const opened: string[] = [];
  const downloaded: string[] = [];
  const oldWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      open: (url: string) => opened.push(url),
      location: { assign: (url: string) => downloaded.push(url) },
    },
  });
  const dispose = fileOpenerStore
    .getState()
    .register("test-editor", ["p1"], () => "https://editor.example/file");
  const request = {
    projectId: "p1",
    chatId: "c1",
    cwd: "/workspace",
    path: "/workspace/src/app.go",
  };
  try {
    openWorkspaceFile(request);
    assert.deepEqual(opened, ["https://editor.example/file"]);
    openWorkspaceFile({ ...request, projectId: "p2" });
    assert.deepEqual(downloaded, [
      "/api/chats/c1/files/download?path=src%2Fapp.go",
    ]);
    openWorkspaceFile({ ...request, path: "/workspace/photo.png" });
    assert.deepEqual(mediaViewerStore.getState().item, {
      name: "photo.png",
      kind: "image",
      url: "/api/chats/c1/media-open?path=%2Fworkspace%2Fphoto.png",
    });
    openWorkspaceFile({ ...request, path: "/workspace/archive.zip" });
    assert.equal(downloaded.length, 2);
    for (const path of ["/etc/passwd", "/workspace/../secret", "/workspace/"])
      openWorkspaceFile({ ...request, path });
    assert.equal(opened.length, 1);
    assert.equal(downloaded.length, 2);
  } finally {
    dispose();
    mediaViewerStore.getState().close();
    if (oldWindow) Object.defineProperty(globalThis, "window", oldWindow);
    else Reflect.deleteProperty(globalThis, "window");
  }
});
