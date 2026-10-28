import { fileManagerUrl } from "./workspacePath.js";

const ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7V5a2 2 0 0 1 2-2h5l2 4h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/></svg>';
const TOKENS = [
  "--fg",
  "--fg-muted",
  "--bg-surface-rgb",
  "--bg-inset-rgb",
  "--accent-blue",
  "--accent-red",
  "--line",
];

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
      const theme = () => {
        const styles = getComputedStyle(document.documentElement);
        frame.contentWindow?.postMessage(
          {
            source: "remote-file-manager-theme",
            tokens: Object.fromEntries(
              TOKENS.map((name) => [name, styles.getPropertyValue(name)]),
            ),
          },
          origin,
        );
      };
      frame.addEventListener("load", theme);
      const observer = new MutationObserver(theme);
      observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ["class", "data-theme", "style"],
      });
      const onMessage = (event) => {
        if (
          event.source !== frame.contentWindow ||
          event.origin !== origin ||
          event.data?.source !== "remote-file-manager" ||
          event.data.type !== "open"
        )
          return;
        const path = event.data.path;
        if (
          typeof path !== "string" ||
          !path ||
          path.startsWith("/") ||
          path.split("/").some((part) => part === ".." || part === ".")
        )
          return;
        remote.files.open({
          chatId: context.chatId,
          projectId: context.projectId,
          cwd: context.cwd,
          path: `/workspace/${path}`,
        });
      };
      window.addEventListener("message", onMessage);
      body.append(frame);
      return () => {
        observer.disconnect();
        window.removeEventListener("message", onMessage);
        frame.removeEventListener("load", theme);
        frame.remove();
      };
    },
  });
}
