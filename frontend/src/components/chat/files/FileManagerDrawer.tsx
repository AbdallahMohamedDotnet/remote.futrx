import { useEffect, useMemo, useState } from "preact/hooks";
import type { FileTree } from "../../../models/files";
import { chatService } from "../../../api/chatService";
import { ChevronsDownUp, ChevronsUpDown, Download, Folder, Loader, RotateCcw, Search, X } from "../../ui/icons";
import { FileTreeNodes } from "./FileTree";
import { collectFolderKeys, filterTree, formatBytes, nodeKey, treeStats } from "./fileMeta";

const DIR_LABELS: Record<string, string> = { ".uploads": "Uploads", ".media": "Media" };

export function FileManagerDrawer({
  chatId,
  open,
  onClose,
}: {
  chatId: string;
  open: boolean;
  onClose: () => void;
}) {
  const [trees, setTrees] = useState<FileTree[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");

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
      const nextTrees = response.trees || [];
      setTrees(nextTrees);
      setTruncated(response.truncated);
      // Auto-expand the first level of each dir so structure is visible at a glance.
      const initial = new Set<string>();
      for (const tree of nextTrees) {
        for (const node of tree.children) {
          if (node.isDir) initial.add(nodeKey(tree.dir, node.path));
        }
      }
      setExpanded(initial);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  function toggle(key: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function expandAll() {
    const all = new Set<string>();
    for (const tree of trees) collectFolderKeys(tree.dir, tree.children, all);
    setExpanded(all);
  }

  const stats = useMemo(() => {
    let files = 0;
    let size = 0;
    for (const tree of trees) {
      const s = treeStats(tree.children);
      files += s.files;
      size += s.size;
    }
    return { files, size };
  }, [trees]);

  // While searching, prune the trees and force-open every matching branch.
  const view = useMemo(() => {
    if (!query.trim()) return { trees, forced: null as Set<string> | null };
    const forced = new Set<string>();
    const filtered = trees.map((tree) => ({
      ...tree,
      children: filterTree(tree.dir, tree.children, query, forced),
    }));
    return { trees: filtered, forced };
  }, [trees, query]);

  const effectiveExpanded = view.forced ? new Set([...expanded, ...view.forced]) : expanded;
  const allExpanded = isFullyExpanded(trees, expanded);

  return (
    <aside
      class={`relative z-20 h-full flex-none overflow-hidden bg-[#101318] border-l border-white/10 shadow-2xl
              transition-[width,opacity] duration-200 ease-out ${open ? "opacity-100" : "opacity-0 border-l-0 pointer-events-none"}`}
      style={{ width: open ? "min(560px, calc(100vw - 320px))" : "0px" }}
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
                : `${stats.files} file${stats.files === 1 ? "" : "s"} · ${formatBytes(stats.size)}`}
            </div>
          </div>
          <button
            type="button"
            onClick={() => (allExpanded ? setExpanded(new Set()) : expandAll())}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center"
            title={allExpanded ? "Collapse all" : "Expand all"}
            aria-label={allExpanded ? "Collapse all" : "Expand all"}
          >
            {allExpanded ? <ChevronsDownUp class="w-4 h-4" /> : <ChevronsUpDown class="w-4 h-4" />}
          </button>
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

        <div class="flex-none px-3 md:px-4 py-2 border-b border-white/[0.06]">
          <div class="relative">
            <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-ink-400 pointer-events-none" />
            <input
              type="text"
              value={query}
              onInput={(event) => setQuery((event.currentTarget as HTMLInputElement).value)}
              placeholder="Search files..."
              class="h-8 w-full bg-[#0b0d11] border border-white/10 rounded-md pl-8 pr-8 text-[13px] text-ink-100
                     placeholder:text-ink-500 focus:outline-none focus:border-accent-blue/60"
            />
            {query && (
              <button
                type="button"
                onClick={() => setQuery("")}
                class="absolute right-2 top-1/2 -translate-y-1/2 h-5 w-5 grid place-items-center rounded text-ink-400 hover:text-ink-100"
                aria-label="Clear search"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        </div>

        <div class="flex-1 min-h-0 overflow-y-auto px-2 md:px-3 py-3 space-y-4">
          {error && (
            <div class="text-[13px] text-accent-red bg-accent-red/10 border border-accent-red/30 rounded-md px-3 py-2">
              {error}
            </div>
          )}
          {truncated && (
            <div class="text-[12px] text-amber-300/90 bg-amber-400/10 border border-amber-400/25 rounded-md px-3 py-2">
              This listing is large and was truncated. Download a folder to get everything.
            </div>
          )}
          {view.trees.map((tree) => (
            <section key={tree.dir}>
              <div class="flex items-center gap-2 px-1.5 pb-1">
                <span class="text-[11px] uppercase tracking-wider text-ink-400 font-semibold">
                  {DIR_LABELS[tree.dir] || tree.dir}
                </span>
                <span class="font-mono text-[11px] text-ink-500">{tree.dir}/</span>
                <span class="flex-1" />
                {tree.exists && tree.children.length > 0 && (
                  <a
                    href={chatService.folderDownloadUrl(chatId, tree.dir)}
                    class="inline-flex items-center gap-1 h-6 px-2 rounded text-[11px] text-ink-300 hover:text-accent-blue hover:bg-white/[0.06]"
                    title={`Download all of ${tree.dir} as zip`}
                  >
                    <Download class="w-3.5 h-3.5" />
                    <span>zip</span>
                  </a>
                )}
              </div>
              {tree.children.length === 0 ? (
                <div class="text-[13px] text-ink-400 px-2 py-2">
                  {tree.exists
                    ? query.trim()
                      ? "No matches."
                      : "No files yet."
                    : "Folder not created yet."}
                </div>
              ) : (
                <FileTreeNodes
                  chatId={chatId}
                  dir={tree.dir}
                  nodes={tree.children}
                  depth={0}
                  expanded={effectiveExpanded}
                  onToggle={toggle}
                />
              )}
            </section>
          ))}
        </div>
      </div>
    </aside>
  );
}

/** True when every folder across all trees is currently expanded. */
function isFullyExpanded(trees: FileTree[], expanded: Set<string>): boolean {
  const all = new Set<string>();
  for (const tree of trees) collectFolderKeys(tree.dir, tree.children, all);
  if (all.size === 0) return false;
  for (const key of all) {
    if (!expanded.has(key)) return false;
  }
  return true;
}
