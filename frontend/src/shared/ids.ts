export function randomId(length = 8): string {
  return Math.random().toString(36).slice(2, 2 + length);
}

export function queueId(): string {
  return `${Date.now().toString(36)}-${randomId(6)}`;
}
