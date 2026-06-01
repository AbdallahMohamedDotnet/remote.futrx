import { useState } from "preact/hooks";
import { ExternalLink, Monitor, RotateCcw, X } from "../ui/icons";

export function BrowserDrawer({
  open,
  projectName,
  url,
  onClose,
}: {
  open: boolean;
  projectName: string;
  url: string;
  onClose: () => void;
}) {
  const [reloadKey, setReloadKey] = useState(0);
  const canLoad = !!url;

  return (
    <aside
      class={`absolute inset-y-0 right-0 z-30 w-full sm:w-[min(88vw,980px)] lg:w-[min(64vw,980px)]
              bg-[#0f1014] border-l border-white/10 shadow-2xl flex flex-col min-h-0
              transition-transform duration-200 ease-out
              ${open ? "translate-x-0 pointer-events-auto" : "translate-x-full pointer-events-none"}`}
      aria-hidden={!open}
      aria-label="Browser preview"
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
            <span class={`h-2 w-2 rounded-full flex-none ${canLoad ? "bg-accent-green" : "bg-ink-400"}`} />
          </div>
          <div class="truncate text-[12px] text-ink-300">
            {canLoad ? url : projectName || "No project container"}
          </div>
        </div>

        <button
          type="button"
          onClick={() => setReloadKey((value) => value + 1)}
          disabled={!canLoad}
          class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center disabled:opacity-50 disabled:cursor-not-allowed"
          title="Reload"
          aria-label="Reload browser"
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
            class="h-full w-full border-0 bg-white"
            allow="clipboard-read; clipboard-write"
          />
        ) : (
          <div class="h-full w-full bg-[#0b0d11] grid place-items-center px-6 text-center text-sm text-ink-300">
            No public dev URL found in this chat yet.
          </div>
        )}
      </div>
    </aside>
  );
}
