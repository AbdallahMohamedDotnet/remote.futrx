export type WorkspaceView = "chat" | "settings" | "project-containers";

export interface WorkspaceUiState {
  activeChatId: string | null;
  containerProjectId: string | null;
  sidebarOpen: boolean;
  view: WorkspaceView;
}

export type WorkspaceUiAction =
  | { type: "select-chat"; chatId: string | null }
  | { type: "open-sidebar" }
  | { type: "close-sidebar" }
  | { type: "show-chat" }
  | { type: "show-settings" }
  | { type: "show-project-containers"; projectId: string | null };

class WorkspaceUiStateTransitions {
  createInitial(): WorkspaceUiState {
    return {
      activeChatId: null,
      containerProjectId: null,
      sidebarOpen: false,
      view: "chat",
    };
  }

  readonly reduce = (
    state: WorkspaceUiState,
    action: WorkspaceUiAction
  ): WorkspaceUiState => {
    switch (action.type) {
      case "select-chat":
        return {
          ...state,
          activeChatId: action.chatId,
          sidebarOpen: false,
          view: "chat",
        };
      case "open-sidebar":
        return { ...state, sidebarOpen: true };
      case "close-sidebar":
        return { ...state, sidebarOpen: false };
      case "show-chat":
        return { ...state, view: "chat" };
      case "show-settings":
        return { ...state, view: "settings", sidebarOpen: false };
      case "show-project-containers":
        return {
          ...state,
          containerProjectId: action.projectId,
          view: "project-containers",
          sidebarOpen: false,
        };
    }
  };
}

export const workspaceUiState = new WorkspaceUiStateTransitions();
