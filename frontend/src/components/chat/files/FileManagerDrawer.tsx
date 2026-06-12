import { useEffect, useState } from "preact/hooks";
import type { WorkspaceDirListing } from "../../../models/files";
import { chatService } from "../../../services/chatService";
import { timeAgo } from "../../../lib/format";
import { Download, Folder, Loader, RotateCcw, X } from "../../ui/icons";

const DIR_LABELS: Record<string, string> = {
  ".uploads": "Uploads",
  ".media": "Media",
};

export function FileManagerDrawer({
  chatId,
  open,
  onClose,
}: {
  chatId: string;
  open: boolean;
  onClose: () => void;
}) {
  const [dirs, setDirs] = useState<WorkspaceDirListing[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatId, open]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const response = await chatService.files(chatId);
      setDirs(response.dirs || []);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  const totalFiles = dirs.reduce((n, d) => n + d.files.length, 0);

  return (
    <aside
      class={`relative z-20 h-full flex-none overflow-hidden bg-[#101318] border-l border-white/10 shadow-2xl
              transition-[width,opacity] duration-200 ease-out ${open ? "opacity-100" : "opacity-0 border-l-0 pointer-events-none"}`}
      style={{ width: open ? "min(520px, calc(100vw - 360px))" : "0px" }}
      aria-hidden={!open}
      aria-label="Files"
    >
      <div
        class={`h-full min-h-0 w-full flex flex-col transition-transform duration-200 ease-out ${open ? "translate-x-0" : "translate-x-full"}`}
      >
        <header class="codex-header flex-none bg-[#191a1f] border-b border-white/10 px-3 md:px-4 py-2.5 flex items-center gap-2">
          <div class="h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 grid place-items-center flex-none">
            <Folder class="w-4 h-4 text-accent-blue" />
          </div>
          <div class="min-w-0 flex-1">
            <h2 class="truncate text-[15px] md:text-base font-semibold text-ink-50">Files</h2>
            <div class="truncate text-[12px] text-ink-300">
              {loading
                ? "Loading..."
                : `${totalFiles} file${totalFiles === 1 ? "" : "s"} in .uploads and .media`}
            </div>
          </div>
          <button
            type="button"
            onClick={() => void load()}
            disabled={loading}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center disabled:cursor-wait"
            title="Refresh"
            aria-label="Refresh files"
          >
            {loading ? <Loader class="w-4 h-4 animate-spin" /> : <RotateCcw class="w-4 h-4" />}
          </button>
          <button
            type="button"
            onClick={onClose}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center"
            title="Close files"
            aria-label="Close files"
          >
            <X class="w-4 h-4" />
          </button>
        </header>

        <div class="flex-1 min-h-0 overflow-y-auto px-3 md:px-4 py-3 space-y-4">
          {error && (
            <div class="text-[13px] text-accent-red bg-accent-red/10 border border-accent-red/30 rounded-md px-3 py-2">
              {error}
            </div>
          )}
          {dirs.map((listing) => (
            <section key={listing.dir}>
              <div class="flex items-center gap-2 px-1 pb-1.5 text-[11px] uppercase tracking-wider text-ink-400 font-semibold">
                <span>{DIR_LABELS[listing.dir] || listing.dir}</span>
                <span class="font-mono lowercase tracking-normal text-ink-500">{listing.dir}/</span>
              </div>
              {listing.files.length === 0 ? (
                <div class="text-[13px] text-ink-400 px-1 py-2">
                  {listing.exists ? "No files yet." : "Folder not created yet."}
                </div>
              ) : (
                <ul class="space-y-0.5">
                  {listing.files.map((file) => (
                    <li
                      key={file.name}
                      class="group flex items-center gap-2 rounded px-2 py-1.5 hover:bg-white/[0.04] border border-transparent"
                    >
                      <div class="flex-1 min-w-0">
                        <div class="text-[13px] text-ink-100 truncate">{file.name}</div>
                        <div class="text-[11px] text-ink-400">
                          {formatBytes(file.size)} · {timeAgo(file.modTime)}
                        </div>
                      </div>
                      <a
                        href={chatService.fileDownloadUrl(chatId, listing.dir, file.name)}
                        download={file.name}
                        class="h-8 w-8 grid place-items-center rounded-md text-ink-300 hover:text-accent-blue hover:bg-white/[0.08] flex-none"
                        title={`Download ${file.name}`}
                        aria-label={`Download ${file.name}`}
                      >
                        <Download class="w-4 h-4" />
                      </a>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          ))}
        </div>
      </div>
    </aside>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(value < 10 ? 1 : 0)} ${units[i]}`;
}
