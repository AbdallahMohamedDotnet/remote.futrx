export function shortPath(path: string | undefined): string {
  if (!path) return "";
  if (path.startsWith("/root/")) return "~" + path.slice(5);
  return path;
}

export function truncate(value: string, max: number): string {
  if (value.length <= max) return value;
  return value.slice(0, max) + `\n\n... (${value.length - max} more characters truncated)`;
}
