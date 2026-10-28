import { fileManagerUrl } from "./workspacePath.js";
import { codeServerRunning, openFileWithoutCore } from "./fileOpening.js";

const ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
const REFRESH_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/></svg>';
// Remote's theme tokens the page needs to look like the rest of the app.
const TOKENS = [
  "--fg-bright",
  "--fg",
  "--fg-dim",
  "--fg-muted",
  "--fg-subtle",
  "--fg-faint",
  "--bg-surface-rgb",
  "--bg-inset-rgb",
  "--line",
  "--tint",
  "--tint-strong",
  "--accent-blue",
  "--accent-green",
  "--accent-red",
  "--accent-yellow",
  "--accent-orange",
  "--accent-purple",
  "--scrollbar-thumb",
  "--scrollbar-thumb-hover",
  "--selection-bg",
];
// Remote pads its panes less below its `md` breakpoint.
const COMPACT = "(max-width: 767px)";

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
      const compact = window.matchMedia(COMPACT);
      const theme = () => {
        const styles = getComputedStyle(document.documentElement);
        send({
          type: "theme",
          tokens: Object.fromEntries(
            TOKENS.map((name) => [name, styles.getPropertyValue(name).trim()]),
          ),
          compact: compact.matches,
        });
      };
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
      frame.addEventListener("load", theme);
      compact.addEventListener("change", theme);
      const observer = new MutationObserver(theme);
      observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ["class", "data-theme", "style"],
      });
      // Only Remote versions without `remote.files.open` need to know.
      let codeServer = false;
      const refreshCodeServer = () => {
        if (remote.files?.open) return;
        void codeServerRunning(context.projectId).then((running) => {
          codeServer = running;
        });
      };
      refreshCodeServer();
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
        const request = {
          chatId: context.chatId,
          projectId: context.projectId,
          cwd: context.cwd,
          path: `/workspace/${path}`,
        };
        if (!remote.files?.open) {
          const result = openFileWithoutCore(request, codeServer);
          if (result.blocked) send({ type: "blocked", path, url: result.blocked });
          // Pick up Code Server being installed or stopped for the next click.
          refreshCodeServer();
          return;
        }
        // Remote opened the editor after the click in the iframe had ended,
        // and the browser refused the tab; let the user click through.
        if (remote.files.open(request) === "blocked") {
          const url = remote.files.editorUrl?.(request);
          if (url) send({ type: "blocked", path, url });
        }
      };
      window.addEventListener("message", onMessage);
      body.append(frame);
      return () => {
        observer.disconnect();
        compact.removeEventListener("change", theme);
        window.removeEventListener("message", onMessage);
        frame.removeEventListener("load", theme);
        frame.remove();
      };
    },
  });
}
