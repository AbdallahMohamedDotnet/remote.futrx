import type { MediaViewerItem } from "../../../models/files";
import { mediaViewerStore } from "../../stores/media/mediaViewerStore";
import { useZustandStore } from "../useZustandStore";

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
  const item = useZustandStore(mediaViewerStore, (state) => state.item);
  const close = useZustandStore(mediaViewerStore, (state) => state.close);

  return { item, close };
}
