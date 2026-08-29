import { useEffect, useState } from "preact/hooks";
import { workspaceApi } from "../../../api/workspaceApi";
import type { WorkspaceSnapshot } from "../../../models/workspace";
import { WorkspaceStore } from "../../stores/workspaceStore";

// One feed for the whole app. The concrete socket is wired here rather than
// inside the store so the store stays free of the api layer and testable.
const workspaceStore = new WorkspaceStore(workspaceApi.subscribe);

/** The chats and projects the server is pushing. */
export function useWorkspaceData(enabled: boolean): WorkspaceSnapshot {
  const [snapshot, setSnapshot] = useState(workspaceStore.snapshot);

  useEffect(() => {
    const unsubscribe = workspaceStore.subscribe(setSnapshot);
    workspaceStore.setConnected(enabled);
    // The store may have moved between the initial read above and this
    // subscription.
    setSnapshot(workspaceStore.snapshot);
    return () => {
      unsubscribe();
      workspaceStore.setConnected(false);
    };
  }, [enabled]);

  return snapshot;
}
