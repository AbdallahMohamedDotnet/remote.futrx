import type { MediaViewerItem } from "../../../models/files";
import { Listeners } from "../listeners.ts";

type Listener = (item: MediaViewerItem | null) => void;

// App-wide in-app media viewer. Any surface (file manager rows, chat message
// links) opens media here instead of navigating away; a single overlay host
// subscribes and renders the current item.
class MediaViewerStore {
  private item: MediaViewerItem | null = null;
  private readonly listeners = new Listeners<MediaViewerItem | null>();

  get current(): MediaViewerItem | null {
    return this.item;
  }

  open(item: MediaViewerItem): void {
    this.item = item;
    this.listeners.emit(this.item);
  }

  close(): void {
    if (this.item === null) return;
    this.item = null;
    this.listeners.emit(this.item);
  }

  subscribe(listener: Listener): () => void {
    return this.listeners.add(listener);
  }
}

export const mediaViewerStore = new MediaViewerStore();
