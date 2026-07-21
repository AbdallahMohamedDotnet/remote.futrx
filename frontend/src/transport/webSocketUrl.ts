import { WEB_SOCKET_PROTOCOLS } from "../config/transport";

export function webSocketUrl(path: `/${string}`): string {
  const protocol = location.protocol === "https:"
    ? WEB_SOCKET_PROTOCOLS.secure
    : WEB_SOCKET_PROTOCOLS.insecure;
  return `${protocol}//${location.host}${path}`;
}
