import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type { FileNode } from "../../../models/files";
import { chatApi } from "../../../api/chatApi";
import { Folder, Loader, RotateCcw, Search, X } from "../../primitives/icons";
import { FileTreeNodes, SearchResultRow, type TreeState } from "./FileTree";

export function FileManagerDrawer({
  chatId,
  open,
  onClose,
}: {
  chatId: string;
  open: boolean;
  onClose: () => void;
}) {
  // Lazily-loaded directory cache. "" is the workspace root.
  const [childrenByDir, setChildrenByDir] = useState<Map<string, FileNode[]>>(new Map());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState<Set<string>>(new Set());
  const [errorByDir, setErrorByDir] = useState<Map<string, string>>(new Map());
  const [truncatedDirs, setTruncatedDirs] = useState<Set<string>>(new Set());
  const [rootLoading, setRootLoading] = useState(false);

  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<FileNode[] | null>(null);
  const [searchTruncated, setSearchTruncated] = useState(false);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  // Bumped on every (re)load so in-flight directory fetches from a previous
  // chat/refresh can't write stale results into the cache.
  const loadToken = useRef(0);

  useEffect(() => {
    if (!open) return;
    void reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatId, open]);

  async function reset() {
    loadToken.current += 1;
    setChildrenByDir(new Map());
    setExpanded(new Set());
    setLoading(new Set());
    setErrorByDir(new Map());
    setTruncatedDirs(new Set());
    setQuery("");
    setSearchResults(null);
    await loadDir("", loadToken.current);
  }

  async function loadDir(path: string, token: number) {
    if (path === "") setRootLoading(true);
    setLoading((prev) => new Set(prev).add(path));
    try {
      const listing = await chatApi.listDir(chatId, path);
      if (token !== loadToken.current) return;
      setChildrenByDir((prev) => new Map(prev).set(path, listing.entries || []));
      setErrorByDir((prev) => {
        const next = new Map(prev);
        next.delete(path);
        return next;
      });
      setTruncatedDirs((prev) => {
        const next = new Set(prev);
        if (listing.truncated) next.add(path);
        else next.delete(path);
        return next;
      });
    } catch (err) {
      if (token !== loadToken.current) return;
      setErrorByDir((prev) => new Map(prev).set(path, (err as Error).message));
    } finally {
      if (token === loadToken.current) {
        if (path === "") setRootLoading(false);
        setLoading((prev) => {
          const next = new Set(prev);
          next.delete(path);
          return next;
        });
      }
    }
  }

  function toggle(path: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
        if (!childrenByDir.has(path) && !loading.has(path)) {
          void loadDir(path, loadToken.current);
        }
      }
      return next;
    });
  }

  // Debounced server-side search across the whole workspace.
  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      setSearchResults(null);
      setSearchError(null);
      setSearching(false);
      return;
    }
    let active = true;
    setSearching(true);
    const timer = setTimeout(async () => {
      try {
        const result = await chatApi.searchFiles(chatId, q);
        if (!active) return;
        setSearchResults(result.entries || []);
        setSearchTruncated(result.truncated);
        setSearchError(null);
      } catch (err) {
        if (!active) return;
        setSearchError((err as Error).message);
        setSearchResults([]);
      } finally {
        if (active) setSearching(false);
      }
    }, 250);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [chatId, query]);

  const rootEntries = childrenByDir.get("") ?? [];
  const rootError = errorByDir.get("");
  const anyTruncated = truncatedDirs.size > 0;

  const treeState = useMemo<TreeState>(
    () => ({ chatId, expanded, loading, childrenByDir, errorByDir, onToggle: toggle }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [chatId, expanded, loading, childrenByDir, errorByDir]
  );

  const subtitle = searchResults
    ? `${searchResults.length} result${searchResults.length === 1 ? "" : "s"}${searchTruncated ? "+" : ""}`
    : rootLoading
      ? "Loading..."
      : "workspace";

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
            <div class="truncate text-[12px] text-ink-300 font-mono">{subtitle}</div>
          </div>
          <button
            type="button"
            onClick={() => void reset()}
            disabled={rootLoading}
            class="h-9 w-9 rounded-md bg-white/5 hover:bg-white/[0.09] border border-white/10 text-ink-200 grid place-items-center disabled:cursor-wait"
            title="Refresh"
            aria-label="Refresh files"
          >
            {rootLoading ? <Loader class="w-4 h-4 animate-spin" /> : <RotateCcw class="w-4 h-4" />}
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
              placeholder="Search all files..."
              class="h-8 w-full bg-[#0b0d11] border border-white/10 rounded-md pl-8 pr-8 text-[13px] text-ink-100
                     placeholder:text-ink-500 focus:outline-none focus:border-accent-blue/60"
            />
            {(query || searching) && (
              <button
                type="button"
                onClick={() => setQuery("")}
                class="absolute right-2 top-1/2 -translate-y-1/2 h-5 w-5 grid place-items-center rounded text-ink-400 hover:text-ink-100"
                aria-label="Clear search"
              >
                {searching ? <Loader class="w-3.5 h-3.5 animate-spin" /> : <X class="w-3.5 h-3.5" />}
              </button>
            )}
          </div>
        </div>

        <div class="flex-1 min-h-0 overflow-y-auto px-2 md:px-3 py-3">
          {searchResults !== null ? (
            <SearchView
              chatId={chatId}
              results={searchResults}
              truncated={searchTruncated}
              searching={searching}
              error={searchError}
            />
          ) : (
            <BrowseView
              rootEntries={rootEntries}
              rootError={rootError}
              rootLoading={rootLoading}
              anyTruncated={anyTruncated}
              treeState={treeState}
            />
          )}
        </div>
      </div>
    </aside>
  );
}

function BrowseView({
  rootEntries,
  rootError,
  rootLoading,
  anyTruncated,
  treeState,
}: {
  rootEntries: FileNode[];
  rootError?: string;
  rootLoading: boolean;
  anyTruncated: boolean;
  treeState: TreeState;
}) {
  return (
    <>
      {rootError && (
        <div class="mb-3 text-[13px] text-accent-red bg-accent-red/10 border border-accent-red/30 rounded-md px-3 py-2">
          {rootError}
        </div>
      )}
      {anyTruncated && (
        <div class="mb-3 text-[12px] text-amber-300/90 bg-amber-400/10 border border-amber-400/25 rounded-md px-3 py-2">
          A folder was large and its listing was truncated. Use search or download the folder as a zip to get everything.
        </div>
      )}
      {!rootLoading && !rootError && rootEntries.length === 0 ? (
        <div class="text-[13px] text-ink-400 px-2 py-2">This workspace is empty.</div>
      ) : (
        <FileTreeNodes nodes={rootEntries} depth={0} state={treeState} />
      )}
    </>
  );
}

function SearchView({
  chatId,
  results,
  truncated,
  searching,
  error,
}: {
  chatId: string;
  results: FileNode[];
  truncated: boolean;
  searching: boolean;
  error: string | null;
}) {
  return (
    <>
      {error && (
        <div class="mb-3 text-[13px] text-accent-red bg-accent-red/10 border border-accent-red/30 rounded-md px-3 py-2">
          {error}
        </div>
      )}
      {truncated && (
        <div class="mb-3 text-[12px] text-amber-300/90 bg-amber-400/10 border border-amber-400/25 rounded-md px-3 py-2">
          Showing the first matches only — refine your search to narrow it down.
        </div>
      )}
      {!searching && !error && results.length === 0 ? (
        <div class="text-[13px] text-ink-400 px-2 py-2">No matches.</div>
      ) : (
        <ul>
          {results.map((node) => (
            <SearchResultRow key={node.path} chatId={chatId} node={node} />
          ))}
        </ul>
      )}
    </>
  );
}
