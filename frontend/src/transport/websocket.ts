export function webSocketUrl(path: `/${string}`): string {
  const protocol = location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${location.host}${path}`;
}

export function chatWebSocketUrl(chatId: string, sinceSeq = 0): string {
  const url = webSocketUrl(`/ws/chat/${encodeURIComponent(chatId)}`);
  return sinceSeq > 0 ? `${url}?since=${sinceSeq}` : url;
}

export function workspaceWebSocketUrl(): string {
  return webSocketUrl("/ws/workspace");
}

export function claudeAuthWebSocketUrl(): string {
  return webSocketUrl("/ws/claude/auth-status");
}

export function codexAuthWebSocketUrl(): string {
  return webSocketUrl("/ws/codex/auth-status");
}

export function kimiAuthWebSocketUrl(): string {
  return webSocketUrl("/ws/kimi/auth-status");
}

export function terminalWebSocketUrl(chatId: string): string {
  return webSocketUrl(`/ws/terminal?chat=${encodeURIComponent(chatId)}`);
}
