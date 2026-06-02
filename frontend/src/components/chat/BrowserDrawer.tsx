import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type { ContainerApp } from "../../models/project";
import { ExternalLink, Loader, Monitor, RotateCcw, X } from "../ui/icons";

const browserWidthKey = "remote.futrx.browserDrawerWidth";
const defaultBrowserWidth = 720;
const minBrowserWidth = 360;
const maxBrowserWidth = 1100;
const minChatWidth = 360;

function clampWidth(width: number, maxWidth = maxBrowserWidth): number {
  return Math.min(Math.max(width, minBrowserWidth), Math.max(minBrowserWidth, maxWidth));
}

function readBrowserWidth(): number {
  if (typeof window === "undefined") return defaultBrowserWidth;
  const stored = Number(window.localStorage.getItem(browserWidthKey));
  return Number.isFinite(stored) ? clampWidth(stored) : defaultBrowserWidth;
}

function buildUrl(slug: string, port: number | null): string {
  if (!slug || !port) return "";
  return `https://${slug}--${port}.dev.remote.futrx.dev`;
}

export function BrowserDrawer({
  open,
  projectName,
  projectSlug,
  apps,
  appsLoading,
  selectedPort,
  onSelectPort,
  onRefreshApps,
  onClose,
}: {
  open: boolean;
  projectName: string;
  projectSlug: string;
  apps: ContainerApp[];
  appsLoading: boolean;
  selectedPort: number | null;
  onSelectPort: (port: number | null) => void;
  onRefreshApps: () => void;
  onClose: () => void;
}) {
  const [reloadKey, setReloadKey] = useState(0);
  const [browserWidth, setBrowserWidth] = useState(readBrowserWidth);
  const [resizing, setResizing] = useState(false);
  const asideRef = useRef<HTMLElement>(null);

  const url = useMemo(() => buildUrl(projectSlug, selectedPort), [projectSlug, selectedPort]);
  const canLoad = !!url;

  useEffect(() => {
    window.localStorage.setItem(browserWidthKey, String(browserWidth));
  }, [browserWidth]);

  useEffect(() => {
    if (!open) return;
    function clampToContainer() {
      const container = asideRef.current?.parentElement;
      const bounds = container?.getBoundingClientRect();
      if (!bounds) return;
      const availableWidth = Math.min(maxBrowserWidth, Math.max(minBrowserWidth, bounds.width - minChatWidth));
      setBrowserWidth((width) => clampWidth(width, availableWidth));
    }
    clampToContainer();
    window.addEventListener("resize", clampToContainer);
    return () => window.removeEventListener("resize", clampToContainer);
  }, [open]);

  function handleResizeStart(event: PointerEvent) {
    if (event.button !== 0) return;
    event.preventDefault();
    setResizing(true);

    const previousCursor = document.body.style.cursor;
    const previousUserSelect = document.body.style.userSelect;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";

    function finishResize() {
      setResizing(false);
      document.body.style.cursor = previousCursor;
      document.body.style.userSelect = previousUserSelect;
      window.removeEventListener("pointermove", resize);
      window.removeEventListener("pointerup", finishResize);
      window.removeEventListener("pointercancel", finishResize);
    }

    function resize(moveEvent: PointerEvent) {
      const container = asideRef.current?.parentElement;
      const bounds = container?.getBoundingClientRect();
      if (!bounds) return;

      const availableWidth = Math.min(maxBrowserWidth, Math.max(minBrowserWidth, bounds.width - minChatWidth));
      const next = bounds.right - moveEvent.clientX;
      setBrowserWidth(clampWidth(next, availableWidth));
    }

    window.addEventListener("pointermove", resize, { passive: false });
    window.addEventListener("pointerup", finishResize);
    window.addEventListener("pointercancel", finishResize);
  }

  function appLabel(app: ContainerApp): string {
    const name = app.process?.trim() || "process";
    return `${name} · :${app.port}`;
  }

  return (
    <aside
      ref={asideRef}
      class={`relative z-20 h-full flex-none overflow-hidden bg-[#101318] border-l border-white/10
              ${resizing ? "transition-none" : "transition-[width,opacity] duration-200 ease-out"}
              ${open ? "opacity-100 shadow-2xl" : "opacity-0 border-l-0 shadow-none pointer-events-none"}`}
      style={{
        width: open ? `${browserWidth}px` : "0px",
        maxWidth: open ? `max(${minBrowserWidth}px, calc(100% - ${minChatWidth}px))` : "0px",
      }}
      aria-hidden={!open}
      aria-label="Browser preview"
    >
      <button
        type="button"
        onPointerDown={handleResizeStart}
        class={`hidden sm:block absolute inset-y-0 left-0 z-10 w-2 cursor-col-resize touch-none
                ${resizing ? "bg-accent-blue/35" : "bg-transparent hover:bg-accent-blue/25"}`}
        title="Resize browser preview"
        aria-label="Resize browser preview"
      >
        <span class="absolute left-1/2 top-1/2 h-12 w-0.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white/30" />
      </button>
      <div
        class={`h-full min-h-0 w-full flex flex-col transition-transform duration-200 ease-out
                ${open ? "translate-x-0" : "translate-x-full"}`}
      >
        <header class="codex-header flex-none bg-[#191a1f] border-b border-white/10 px-3 md:px-4 py-2.5 flex items-center gap-2">
          <div class="h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
            <Monitor class="w-4 h-4 text-accent-blue" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 min-w-0">
              <h2 class="truncate text-[15px] md:text-base font-semibold text-ink-50">
                Browser
              </h2>
              <span
                class={`h-2 w-2 rounded-full flex-none ${canLoad ? "bg-accent-green" : "bg-ink-400"}`}
              />
            </div>
            {apps.length > 0 ? (
              <select
                value={selectedPort ?? ""}
                onChange={(e) => {
                  const v = (e.target as HTMLSelectElement).value;
                  onSelectPort(v ? Number(v) : null);
                }}
                class="mt-0.5 max-w-full truncate bg-transparent text-[12px] text-ink-300 hover:text-ink-100 focus:outline-none focus:text-ink-100 cursor-pointer"
                title={url || "Pick a running app"}
              >
                {apps.map((app) => (
                  <option key={app.port} value={app.port} class="bg-[#191a1f] text-ink-100">
                    {appLabel(app)}
                  </option>
                ))}
              </select>
            ) : (
              <div class="truncate text-[12px] text-ink-300">
                {appsLoading
                  ? "Looking for running apps…"
                  : projectName
                    ? `No apps listening in ${projectName}`
                    : "No project container"}
              </div>
            )}
          </div>

          <button
            type="button"
            onClick={onRefreshApps}
            disabled={appsLoading}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center disabled:cursor-wait"
            title="Refresh running apps"
            aria-label="Refresh running apps"
          >
            {appsLoading ? <Loader class="w-4 h-4 animate-spin" /> : <RotateCcw class="w-4 h-4" />}
          </button>

          <button
            type="button"
            onClick={() => setReloadKey((value) => value + 1)}
            disabled={!canLoad}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center disabled:opacity-50 disabled:cursor-not-allowed"
            title="Reload iframe"
            aria-label="Reload iframe"
          >
            <RotateCcw class="w-4 h-4" />
          </button>

          {canLoad ? (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center"
              title="Open in new tab"
              aria-label="Open browser in new tab"
            >
              <ExternalLink class="w-4 h-4" />
            </a>
          ) : (
            <button
              type="button"
              disabled
              class="h-9 w-9 rounded-md bg-white/5 border border-white/10 text-ink-200 grid place-items-center opacity-50 cursor-not-allowed"
              title="Open in new tab"
              aria-label="Open browser in new tab"
            >
              <ExternalLink class="w-4 h-4" />
            </button>
          )}

          <button
            type="button"
            onClick={onClose}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center"
            title="Close browser"
            aria-label="Close browser"
          >
            <X class="w-4 h-4" />
          </button>
        </header>

        <div class="flex-1 min-h-0 bg-white">
          {canLoad ? (
            <iframe
              key={`${url}:${reloadKey}`}
              src={url}
              title={`Browser preview for ${projectName || "container"}`}
              class={`h-full w-full border-0 bg-white ${resizing ? "pointer-events-none" : ""}`}
              allow="clipboard-read; clipboard-write"
            />
          ) : (
            <div class="h-full w-full bg-[#0b0d11] grid place-items-center px-6 text-center text-sm text-ink-300">
              <div class="max-w-sm space-y-2">
                <div class="text-ink-200 font-medium">No running apps</div>
                <div class="text-[12.5px] leading-relaxed">
                  Tell the agent to start a dev server (it&apos;ll bind to{" "}
                  <code class="font-mono text-ink-100">0.0.0.0</code> on any port).
                  Click the refresh icon and pick it from the dropdown.
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </aside>
  );
}
