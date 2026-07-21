import type { RefObject } from "preact";
import { BrowserEmptyState } from "./BrowserEmptyState";

export function BrowserFrame({
  canLoad,
  iframeRef,
  iframeUrl,
  reloadKey,
  projectName,
  resizing,
  inspectMode,
  onFrameLoad,
}: {
  canLoad: boolean;
  iframeRef: RefObject<HTMLIFrameElement>;
  iframeUrl: string;
  reloadKey: number;
  projectName: string;
  resizing: boolean;
  inspectMode: boolean;
  onFrameLoad: (enabled: boolean) => void;
}) {
  return (
    <div class="flex-1 min-h-0 bg-white">
      {canLoad ? (
        <iframe
          ref={iframeRef}
          key={`${iframeUrl}:${reloadKey}`}
          src={iframeUrl}
          title={`Browser preview for ${projectName || "container"}`}
          onLoad={() => onFrameLoad(inspectMode)}
          class={`h-full w-full border-0 bg-white ${resizing ? "pointer-events-none" : ""}`}
          allow="clipboard-read; clipboard-write"
        />
      ) : (
        <BrowserEmptyState />
      )}
    </div>
  );
}
