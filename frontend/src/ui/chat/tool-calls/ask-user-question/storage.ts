const LS_KEY_PREFIX = "askq-answered:";

function answeredKey(chatId: string, toolUseId: string): string {
  return `${LS_KEY_PREFIX}${chatId}:${toolUseId}`;
}

export function readAnswered(chatId: string, toolUseId: string): string | null {
  try {
    return localStorage.getItem(answeredKey(chatId, toolUseId));
  } catch {
    return null;
  }
}

export function writeAnswered(chatId: string, toolUseId: string, summary: string) {
  try {
    localStorage.setItem(answeredKey(chatId, toolUseId), summary);
  } catch {}
}

export function clearAnswered(chatId: string, toolUseId: string) {
  try {
    localStorage.removeItem(answeredKey(chatId, toolUseId));
  } catch {}
}
