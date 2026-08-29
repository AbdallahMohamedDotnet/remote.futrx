import type { MediaKind } from "../../models/files";

export interface MediaViewerItem {
  url: string;
  name: string;
  kind: MediaKind;
}

type Listener = (item: MediaViewerItem | null) => void;

// App-wide in-app media viewer. Any surface (file manager rows, chat message
// links) opens media here instead of navigating away; a single overlay host
// subscribes and renders the current item.
class MediaViewerState {
  private item: MediaViewerItem | null = null;
  private readonly listeners = new Set<Listener>();

  get current(): MediaViewerItem | null {
    return this.item;
  }

  open(item: MediaViewerItem): void {
    this.item = item;
    this.emit();
  }

  close(): void {
    if (this.item === null) return;
    this.item = null;
    this.emit();
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private emit(): void {
    for (const listener of this.listeners) listener(this.item);
  }
}

export const mediaViewerState = new MediaViewerState();
