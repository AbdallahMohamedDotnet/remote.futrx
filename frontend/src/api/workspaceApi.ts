import type { ChatMeta } from "../models/chat";
import type { ProjectMeta } from "../models/project";
import { ReconnectingJsonWebSocket } from "../transport/ReconnectingJsonWebSocket";
import { webSocketUrl } from "../transport/websocket";

export type WorkspaceMessage =
  | { type: "workspace.snapshot"; chats: ChatMeta[]; projects: ProjectMeta[] }
  | { type: "chat.upsert"; chat: ChatMeta }
  | { type: "chat.delete"; id: string }
  | { type: "project.upsert"; project: ProjectMeta }
  | { type: "project.delete"; id: string };

export const workspaceApi = {
  subscribe: (onMessage: (message: WorkspaceMessage) => void) => {
    const connection = new ReconnectingJsonWebSocket({
      url: () => webSocketUrl("/ws/workspace"),
      onMessage,
    });
    connection.start();
    return () => connection.stop();
  },
};
