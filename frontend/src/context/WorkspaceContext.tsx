import type { ComponentChildren } from "preact";
import { createContext } from "preact";
import { useContext, useEffect, useReducer } from "preact/hooks";
import type { ChatMeta, CreateChatInput } from "../models/chat";
import type { ProjectMeta } from "../models/project";
import { chatService } from "../services/chatService";
import { projectService } from "../services/projectService";
import { useWorkspaceData } from "../hooks/workspace/useWorkspaceData";
import { useUserSettingsContext } from "./UserSettingsContext";
import {
  initialWorkspaceUiState,
  workspaceUiReducer,
  type WorkspaceUiState,
} from "../state/workspace/reducer";
import {
  isActiveChatMissing,
  selectActiveChat,
  shouldSelectInitialChat,
} from "../state/workspace/selectors";

interface WorkspaceContextValue {
  chats: ChatMeta[];
  projects: ProjectMeta[];
  activeChat: ChatMeta | null;
  ui: WorkspaceUiState;
  selectChat: (chatId: string | null) => void;
  openSidebar: () => void;
  closeSidebar: () => void;
  showChat: () => void;
  showSettings: () => void;
  showProjectContainers: (projectId: string | null) => void;
  refreshChats: () => void;
  refreshProjects: () => void;
  createProject: (name: string) => Promise<ProjectMeta>;
  createChat: (projectId?: string) => Promise<ChatMeta>;
  deleteChat: (chatId: string) => Promise<void>;
  deleteProject: (projectId: string) => Promise<void>;
  reorderProjects: (projectIds: string[]) => Promise<void>;
  startProject: (projectId: string) => Promise<void>;
  stopProject: (projectId: string) => Promise<void>;
}

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

export function WorkspaceProvider({
  enabled,
  children,
}: {
  enabled: boolean;
  children: ComponentChildren;
}) {
  const data = useWorkspaceData(enabled);
  const { settings } = useUserSettingsContext();
  const [ui, dispatch] = useReducer(workspaceUiReducer, initialWorkspaceUiState);
  const activeChat = selectActiveChat(data.chats, ui.activeChatId);

  useEffect(() => {
    const chatId = shouldSelectInitialChat(enabled, ui.activeChatId, data.chats);
    if (chatId) dispatch({ type: "select-chat", chatId });
  }, [data.chats, enabled, ui.activeChatId]);

  useEffect(() => {
    if (isActiveChatMissing(data.chats, ui.activeChatId)) {
      dispatch({ type: "select-chat", chatId: null });
    }
  }, [data.chats, ui.activeChatId]);

  async function createProject(name: string): Promise<ProjectMeta> {
    const project = await projectService.create(name);
    return project;
  }

  async function createChat(projectId?: string): Promise<ChatMeta> {
    const input: CreateChatInput = {
      provider: settings.chat.provider,
      model: settings.chat.model,
      mode: settings.chat.mode,
      reasoningEffort: settings.chat.reasoningEffort,
      ...(projectId ? { projectId } : {}),
    };
    const chat = await chatService.create(input);
    dispatch({ type: "select-chat", chatId: chat.id });
    return chat;
  }

  async function deleteChat(chatId: string) {
    await chatService.delete(chatId);
  }

  async function deleteProject(projectId: string) {
    await projectService.delete(projectId);
  }

  async function reorderProjects(projectIds: string[]) {
    await projectService.reorder(projectIds);
  }

  async function startProject(projectId: string) {
    await projectService.start(projectId);
  }

  async function stopProject(projectId: string) {
    await projectService.stop(projectId);
  }

  return (
    <WorkspaceContext.Provider
      value={{
        chats: data.chats,
        projects: data.projects,
        activeChat,
        ui,
        selectChat: (chatId) => dispatch({ type: "select-chat", chatId }),
        openSidebar: () => dispatch({ type: "open-sidebar" }),
        closeSidebar: () => dispatch({ type: "close-sidebar" }),
        showChat: () => dispatch({ type: "show-chat" }),
        showSettings: () => dispatch({ type: "show-settings" }),
        showProjectContainers: (projectId) =>
          dispatch({ type: "show-project-containers", projectId }),
        refreshChats: data.refreshChats,
        refreshProjects: data.refreshProjects,
        createProject,
        createChat,
        deleteChat,
        deleteProject,
        reorderProjects,
        startProject,
        stopProject,
      }}
    >
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspaceContext(): WorkspaceContextValue {
  const value = useContext(WorkspaceContext);
  if (!value) throw new Error("useWorkspaceContext must be used inside WorkspaceProvider");
  return value;
}
