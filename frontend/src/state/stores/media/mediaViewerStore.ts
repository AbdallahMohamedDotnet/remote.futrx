import type {
  MediaViewerStoreActions,
  MediaViewerStoreState,
} from "../../../models/files";
import { createAppStore } from "../appStore.ts";

// App-wide in-app media viewer. Any surface (file manager rows, chat message
// links) opens media here instead of navigating away; a single overlay host
// subscribes and renders the current item.
export const mediaViewerStore = createAppStore<
  MediaViewerStoreState,
  MediaViewerStoreActions
>(
  { item: null },
  ({ setState }) => ({
    open: (item) => setState({ item }),
    close: () => setState((state) => state.item === null ? state : { item: null }),
  }),
);
