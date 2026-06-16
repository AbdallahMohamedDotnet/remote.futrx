export function buildBrowserUrl(slug: string, port: number | null): string {
  if (!slug || !port) return "";
  return `https://${slug}--${port}.dev.remote.futrx.dev`;
}

export function buildInspectorUrl(url: string): string {
  if (!url || typeof window === "undefined") return "";
  const inspector = new URL("/__remote_inspector", url);
  inspector.searchParams.set("target", "/");
  inspector.searchParams.set("parent", window.location.origin);
  return inspector.toString();
}

// buildGuiUrl points at the in-container noVNC view (Agent Browser),
// served on a fixed port through the same dev-URL proxy as app previews.
export function buildGuiUrl(slug: string, port: number): string {
  const base = buildBrowserUrl(slug, port);
  return base ? `${base}/vnc.html?autoconnect=1&resize=scale&reconnect=1` : "";
}
