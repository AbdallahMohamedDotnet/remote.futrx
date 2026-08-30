import { useEffect } from "preact/hooks";
import { workspaceApi } from "../../../api/workspaceApi";
import type { WorkspaceSnapshot } from "../../../models/workspace";
import { createWorkspaceStore } from "../../stores/workspace/workspaceStore";
import { useAppStore } from "../useAppStore.ts";

// One feed for the whole app. The concrete socket is wired here rather than
// inside the store so the store stays free of the api layer and testable.
const workspaceStore = createWorkspaceStore(workspaceApi.subscribe);

/** The chats and projects the server is pushing. */
export function useWorkspaceData(enabled: boolean): WorkspaceSnapshot {
  const snapshot = useAppStore(workspaceStore, (store) => store.state.snapshot);

  useEffect(() => {
    workspaceStore.getState().actions.setConnected(enabled);
    return () => {
      workspaceStore.getState().actions.setConnected(false);
    };
  }, [enabled]);

  return snapshot;
}
