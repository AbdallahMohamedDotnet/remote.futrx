import { createStore } from "zustand/vanilla";
import type { MediaViewerItem } from "../../../models/files";

interface MediaViewerState {
  item: MediaViewerItem | null;
  open: (item: MediaViewerItem) => void;
  close: () => void;
}

// App-wide in-app media viewer. Any surface (file manager rows, chat message
// links) opens media here instead of navigating away; a single overlay host
// subscribes and renders the current item.
export const mediaViewerStore = createStore<MediaViewerState>()((set) => ({
  item: null,
  open: (item) => set({ item }),
  close: () => set((state) => state.item === null ? state : { item: null }),
}));
