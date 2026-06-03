import type { ContainerApp } from "../../../models/project";
import { Crosshair, ExternalLink, Loader, Monitor, RotateCcw, X } from "../../ui/icons";

export function BrowserDrawerHeader({
  projectName,
  apps,
  appsLoading,
  selectedPort,
  url,
  canLoad,
  inspectMode,
  onSelectPort,
  onRefreshApps,
  onToggleInspectMode,
  onReload,
  onClose,
}: {
  projectName: string;
  apps: ContainerApp[];
  appsLoading: boolean;
  selectedPort: number | null;
  url: string;
  canLoad: boolean;
  inspectMode: boolean;
  onSelectPort: (port: number | null) => void;
  onRefreshApps: () => void;
  onToggleInspectMode: () => void;
  onReload: () => void;
  onClose: () => void;
}) {
  return (
    <header class="codex-header flex-none bg-[#191a1f] border-b border-white/10 px-3 md:px-4 py-2.5 flex items-center gap-2">
      <div class="h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
        <Monitor class="w-4 h-4 text-accent-blue" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 min-w-0">
          <h2 class="truncate text-[15px] md:text-base font-semibold text-ink-50">Browser</h2>
          <span class={`h-2 w-2 rounded-full flex-none ${canLoad ? "bg-accent-green" : "bg-ink-400"}`} />
        </div>
        {apps.length > 0 ? (
          <select
            value={selectedPort ?? ""}
            onChange={(event) => {
              const value = (event.target as HTMLSelectElement).value;
              onSelectPort(value ? Number(value) : null);
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
        onClick={onToggleInspectMode}
        disabled={!canLoad}
        class={`h-9 w-9 rounded-md border grid place-items-center disabled:opacity-50 disabled:cursor-not-allowed
                ${inspectMode
                  ? "bg-accent-blue/[0.18] border-accent-blue/35 text-accent-blue"
                  : "bg-white/5 hover:bg-white/[0.09] border-white/10 text-ink-200"}`}
        title="Inspect element"
        aria-label="Inspect element"
        aria-pressed={inspectMode}
      >
        <Crosshair class="w-4 h-4" />
      </button>

      <button
        type="button"
        onClick={onReload}
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
  );
}

function appLabel(app: ContainerApp): string {
  const name = app.process?.trim() || "process";
  return `${name} · :${app.port}`;
}
