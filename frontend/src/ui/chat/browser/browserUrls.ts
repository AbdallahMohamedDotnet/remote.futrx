export function buildInspectorUrl(url: string): string {
  if (!url || typeof window === "undefined") return "";
  const inspector = new URL("/__remote_inspector", url);
  inspector.searchParams.set("target", "/");
  inspector.searchParams.set("parent", window.location.origin);
  return inspector.toString();
}
