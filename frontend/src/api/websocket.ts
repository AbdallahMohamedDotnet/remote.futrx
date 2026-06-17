function wsBase(): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}`;
}

export function chatWebSocketUrl(chatId: string, sinceSeq = 0): string {
  const url = `${wsBase()}/ws/chat/${encodeURIComponent(chatId)}`;
  return sinceSeq > 0 ? `${url}?since=${sinceSeq}` : url;
}

export function workspaceWebSocketUrl(): string {
  return `${wsBase()}/ws/workspace`;
}

export function codexAuthWebSocketUrl(): string {
  return `${wsBase()}/ws/codex/auth-status`;
}

export function terminalWebSocketUrl(chatId: string): string {
  return `${wsBase()}/ws/terminal?chat=${encodeURIComponent(chatId)}`;
}
