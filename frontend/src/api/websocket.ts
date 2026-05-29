export function chatWebSocketUrl(chatId: string): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/ws/chat/${encodeURIComponent(chatId)}`;
}
