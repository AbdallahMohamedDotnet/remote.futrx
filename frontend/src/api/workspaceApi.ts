import { subscribeToJsonMessages } from "../transport/jsonMessageSubscription";
import type { WorkspaceMessage } from "../types/workspaceApi";
import { WEB_SOCKET_ROUTES } from "../config/routes";

export const workspaceApi = {
  subscribe: (onMessage: (message: WorkspaceMessage) => void) =>
    subscribeToJsonMessages(WEB_SOCKET_ROUTES.workspace, onMessage),
};
