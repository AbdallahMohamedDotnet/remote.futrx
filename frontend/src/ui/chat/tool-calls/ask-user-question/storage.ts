const LS_KEY_PREFIX = "askq-answered:";

export function readAnswered(toolUseId: string): string | null {
  try {
    return localStorage.getItem(LS_KEY_PREFIX + toolUseId);
  } catch {
    return null;
  }
}

export function writeAnswered(toolUseId: string, summary: string) {
  try {
    localStorage.setItem(LS_KEY_PREFIX + toolUseId, summary);
  } catch {}
}
