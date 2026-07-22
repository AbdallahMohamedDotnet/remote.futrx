export function safeReturnTo(rawURL: string, baseURL: string): string {
  if (!rawURL || rawURL.length > 2048) return "";

  try {
    const target = new URL(rawURL);
    const base = new URL(baseURL);
    const allowedHost = target.host === base.host || target.host.endsWith(`.${base.host}`);
    return target.protocol === "https:" && allowedHost ? rawURL : "";
  } catch {
    return "";
  }
}
