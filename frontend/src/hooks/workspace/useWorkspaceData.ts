import { usePoll } from "../shared/usePoll";
import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";
import { chatService } from "../../services/chatService";
import { projectService } from "../../services/projectService";

export function useWorkspaceData(enabled: boolean) {
  const { value: chats, refresh: refreshChats } = usePoll<ChatMeta[]>(
    () => (enabled ? chatService.list() : Promise.resolve([])),
    8000,
    []
  );
  const { value: projects, refresh: refreshProjects } = usePoll<ProjectMeta[]>(
    () => (enabled ? projectService.list() : Promise.resolve([])),
    8000,
    []
  );

  async function refreshAll() {
    await Promise.all([refreshChats(), refreshProjects()]);
  }

  return {
    chats,
    projects,
    refreshChats,
    refreshProjects,
    refreshAll,
  };
}
