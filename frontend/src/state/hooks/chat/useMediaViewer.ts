import { useStore } from "zustand";
import type { MediaViewerItem } from "../../../models/files";
import { mediaViewerStore } from "../../stores/media/mediaViewerStore";

/**
 * The media viewer's current item, as a subscription.
 *
 * The store outlives the component tree, so reading it directly from a
 * component would miss every later change. This is the only supported way to
 * read it; opening media is a command and may be dispatched straight at the
 * store from anywhere, including outside a component.
 */
export function useMediaViewer(): {
  item: MediaViewerItem | null;
  close: () => void;
} {
  const item = useStore(mediaViewerStore, (store) => store.state.item);
  const close = useStore(mediaViewerStore, (store) => store.actions.close);

  return { item, close };
}
