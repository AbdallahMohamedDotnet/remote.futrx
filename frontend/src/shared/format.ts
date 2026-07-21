// Shorten an absolute path by collapsing $HOME to ~.
export function shortenPath(p: string, home = "/root"): string {
  if (!p) return "~";
  if (p === home) return "~";
  if (p.startsWith(home + "/")) return "~" + p.slice(home.length);
  return p;
}

// Format a unix-ms timestamp as a "5m ago" / "2h ago" / absolute-date string.
export function timeAgo(ms: number, now = Date.now()): string {
  const sec = Math.max(0, Math.floor((now - ms) / 1000));
  if (sec < 5) return "just now";
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const days = Math.floor(hr / 24);
  if (days < 7) return `${days}d ago`;
  return new Date(ms).toLocaleDateString();
}

export function formatModelShortLabel(model?: string): string {
  if (!model) return "auto";
  const lower = model.toLowerCase();
  if (lower.includes("fable")) return "fable";
  if (lower.includes("opus")) return "opus";
  if (lower.includes("sonnet")) return "sonnet";
  if (lower.includes("haiku")) return "haiku";
  return model;
}
