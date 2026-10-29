import { codeServerRunning, openFileWithoutCore } from "./fileOpening.js";

// Each mounted drawer owns its latest Code Server check. A click stays
// synchronous so the browser can still allow a new editor tab.
export function createFileOpener(remote, context, onBlocked) {
  let codeServer = false;
  const refreshCodeServer = () => {
    if (remote.files?.open) return;
    void codeServerRunning(context.projectId).then((running) => {
      codeServer = running;
    });
  };
  refreshCodeServer();

  return (path) => {
    const request = {
      chatId: context.chatId,
      projectId: context.projectId,
      cwd: context.cwd,
      path: `/workspace/${path}`,
    };
    if (!remote.files?.open) {
      const result = openFileWithoutCore(request, codeServer);
      if (result.blocked) onBlocked(path, result.blocked);
      // Pick up Code Server being installed or stopped for the next click.
      refreshCodeServer();
      return;
    }
    if (remote.files.open(request) === "blocked") {
      const url = remote.files.editorUrl?.(request);
      if (url) onBlocked(path, url);
    }
  };
}
