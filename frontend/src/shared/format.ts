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

// Sidebar rows show age in one glanceable token ("3h", "5d", "Mar 4") so the
// column width never shifts between rows.
export function shortTimeAgo(ms: number, now = Date.now()): string {
  if (!ms) return "";
  const sec = Math.max(0, Math.floor((now - ms) / 1000));
  if (sec < 60) return "now";
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h`;
  const days = Math.floor(hr / 24);
  if (days < 7) return `${days}d`;
  if (days < 365) return `${Math.floor(days / 7)}w`;
  return `${Math.floor(days / 365)}y`;
}
