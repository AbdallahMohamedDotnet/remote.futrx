import { ReconnectingJsonWebSocket } from "./reconnectingJsonSocket";
import { webSocketUrl } from "./webSocketUrl";
import type { ApplicationPath } from "../types/transport";

export function subscribeToJsonMessages<TMessage>(
  path: ApplicationPath,
  onMessage: (message: TMessage) => void
): () => void {
  const connection = new ReconnectingJsonWebSocket<TMessage>({
    resolveUrl: () => webSocketUrl(path),
    onMessage,
  });
  connection.start();
  return () => connection.stop();
}
