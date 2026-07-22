import { useCallback, useEffect, useMemo, useReducer, useRef } from "preact/hooks";
import { chatFilesApi } from "../../../api/chat/chatFilesApi";
import type { FileNode } from "../../../models/files";
import { workspaceFileBrowserState } from "../../chat/workspaceFileBrowserState";

export interface WorkspaceFileTreeState {
  expanded: Set<string>;
  loading: Set<string>;
  childrenByDir: Map<string, FileNode[]>;
  errorByDir: Map<string, string>;
  onToggle: (path: string) => void;
  downloadUrl: (node: FileNode) => string;
}

export function useWorkspaceFileBrowser({ chatId, active }: { chatId: string; active: boolean }) {
  const [state, dispatch] = useReducer(
    workspaceFileBrowserState.reduce,
    workspaceFileBrowserState.createInitial()
  );
  const stateRef = useRef(state);
  const loadToken = useRef(0);
  stateRef.current = state;

  const loadDirectory = useCallback(
    async (path: string, token: number) => {
      dispatch({ type: "directory-load-started", path });
      try {
        const listing = await chatFilesApi.listDir(chatId, path);
        if (token !== loadToken.current) return;
        dispatch({
          type: "directory-load-succeeded",
          path,
          entries: listing.entries || [],
          truncated: listing.truncated,
        });
      } catch (error) {
        if (token !== loadToken.current) return;
        dispatch({ type: "directory-load-failed", path, error: (error as Error).message });
      }
    },
    [chatId]
  );

  const reset = useCallback(async () => {
    const token = loadToken.current + 1;
    loadToken.current = token;
    dispatch({ type: "reset" });
    await loadDirectory("", token);
  }, [loadDirectory]);

  useEffect(() => {
    if (!active) return;
    void reset();
  }, [active, reset]);

  const toggleDirectory = useCallback(
    (path: string) => {
      const current = stateRef.current;
      const opening = !current.expanded.has(path);
      dispatch({ type: "directory-toggled", path });
      if (opening && !current.childrenByDir.has(path) && !current.loading.has(path)) {
        void loadDirectory(path, loadToken.current);
      }
    },
    [loadDirectory]
  );

  useEffect(() => {
    const query = state.query.trim();
    if (query.length < 2) {
      dispatch({ type: "search-idle" });
      return;
    }

    let activeSearch = true;
    dispatch({ type: "search-started" });
    const timer = setTimeout(async () => {
      try {
        const result = await chatFilesApi.searchFiles(chatId, query);
        if (!activeSearch) return;
        dispatch({
          type: "search-succeeded",
          entries: result.entries || [],
          truncated: result.truncated,
        });
      } catch (error) {
        if (!activeSearch) return;
        dispatch({ type: "search-failed", error: (error as Error).message });
      }
    }, 250);
    return () => {
      activeSearch = false;
      clearTimeout(timer);
    };
  }, [chatId, state.query]);

  const setQuery = useCallback((query: string) => {
    dispatch({ type: "query-changed", query });
  }, []);
  const downloadUrl = useCallback(
    (node: FileNode) =>
      node.isDir
        ? chatFilesApi.folderDownloadUrl(chatId, node.path)
        : chatFilesApi.fileDownloadUrl(chatId, node.path),
    [chatId]
  );
  const treeState = useMemo<WorkspaceFileTreeState>(
    () => ({
      expanded: state.expanded,
      loading: state.loading,
      childrenByDir: state.childrenByDir,
      errorByDir: state.errorByDir,
      onToggle: toggleDirectory,
      downloadUrl,
    }),
    [
      state.expanded,
      state.loading,
      state.childrenByDir,
      state.errorByDir,
      toggleDirectory,
      downloadUrl,
    ]
  );

  return {
    query: state.query,
    setQuery,
    reset,
    rootEntries: state.childrenByDir.get("") ?? [],
    rootError: state.errorByDir.get(""),
    rootLoading: state.rootLoading,
    anyTruncated: state.truncatedDirs.size > 0,
    searchResults: state.searchResults,
    searchTruncated: state.searchTruncated,
    searching: state.searching,
    searchError: state.searchError,
    treeState,
    downloadUrl,
  };
}
