import { usePoll } from "../shared/usePoll";
import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";
import { chatService } from "../../services/chatService";
import { projectService } from "../../services/projectService";

export function useWorkspaceData(enabled: boolean) {
  const { value: chats, refresh: refreshChats } = usePoll<ChatMeta[]>(
    () => (enabled ? chatService.list() : Promise.resolve([])),
    8000,
    [],
    { equals: sameChats }
  );
  const { value: projects, refresh: refreshProjects } = usePoll<ProjectMeta[]>(
    () => (enabled ? projectService.list() : Promise.resolve([])),
    8000,
    [],
    { equals: sameProjects }
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

function sameChats(a: ChatMeta[], b: ChatMeta[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    const left = a[i];
    const right = b[i];
    if (
      left.id !== right.id ||
      left.title !== right.title ||
      left.claudeSessionId !== right.claudeSessionId ||
      left.tmuxSession !== right.tmuxSession ||
      left.cwd !== right.cwd ||
      left.createdAt !== right.createdAt ||
      left.lastMessageAt !== right.lastMessageAt ||
      left.model !== right.model ||
      left.mode !== right.mode ||
      left.projectId !== right.projectId
    ) {
      return false;
    }
  }
  return true;
}

function sameProjects(a: ProjectMeta[], b: ProjectMeta[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    const left = a[i];
    const right = b[i];
    if (
      left.id !== right.id ||
      left.name !== right.name ||
      left.slug !== right.slug ||
      left.cwd !== right.cwd ||
      left.containerName !== right.containerName ||
      left.status !== right.status ||
      left.errorMsg !== right.errorMsg ||
      left.createdAt !== right.createdAt ||
      left.updatedAt !== right.updatedAt
    ) {
      return false;
    }
  }
  return true;
}
