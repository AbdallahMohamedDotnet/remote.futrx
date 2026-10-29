import { fileManagerUrl } from "./workspacePath.js";
import { createFileOpener } from "./fileOpener.js";
import { syncDrawerTheme } from "./drawerTheme.js";

const ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
const REFRESH_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/></svg>';

export default function activate(remote) {
  remote.ui.registerDrawer({
    id: "files",
    title: "Files",
    label: "Workspace files",
    icon: ICON,
    when: (target) =>
      Boolean(
        target.projectId &&
          fileManagerUrl(
            target.cwd,
            target.chatId,
            remote.application.web?.subdomain,
          ),
      ),
    mount: (body, context) => {
      const url = fileManagerUrl(
        context.cwd,
        context.chatId,
        remote.application.web?.subdomain,
      );
      if (!url) {
        context.setStatus({
          label: "Unavailable for this chat",
          active: false,
        });
        return;
      }
      const origin = new URL(url).origin;
      const frame = document.createElement("iframe");
      frame.title = "Workspace files";
      frame.src = url;
      frame.style.cssText =
        "display:block;width:100%;height:100%;border:0;background:transparent";
      const send = (message) =>
        frame.contentWindow?.postMessage(
          { source: "remote-file-manager-host", ...message },
          origin,
        );
      const showState = ({ subtitle, loading }) => {
        context.setStatus({ label: subtitle });
        context.setActions?.([
          {
            id: "refresh",
            label: "Refresh files",
            icon: REFRESH_ICON,
            busy: loading,
            onClick: () => send({ type: "refresh" }),
          },
        ]);
      };
      showState({ subtitle: "Loading...", loading: true });
      const stopTheme = syncDrawerTheme(frame, send);
      const openFile = createFileOpener(remote, context, (path, url) =>
        send({ type: "blocked", path, url }),
      );
      const onMessage = (event) => {
        if (
          event.source !== frame.contentWindow ||
          event.origin !== origin ||
          event.data?.source !== "remote-file-manager"
        )
          return;
        if (event.data.type === "state") {
          showState({
            subtitle: String(event.data.subtitle ?? ""),
            loading: Boolean(event.data.loading),
          });
          return;
        }
        if (event.data.type !== "open") return;
        const path = event.data.path;
        if (
          typeof path !== "string" ||
          !path ||
          path.startsWith("/") ||
          path.split("/").some((part) => part === ".." || part === ".")
        )
          return;
        openFile(path);
      };
      window.addEventListener("message", onMessage);
      body.append(frame);
      return () => {
        stopTheme();
        window.removeEventListener("message", onMessage);
        frame.remove();
      };
    },
  });
}
