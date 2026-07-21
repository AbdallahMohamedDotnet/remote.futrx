import { useCallback, useEffect, useState } from "preact/hooks";
import { workspaceApi, type WorkspaceMessage } from "../../../api/workspaceApi";
import type { ChatMeta } from "../../../models/chat";
import type { ProjectMeta } from "../../../models/project";

export function useWorkspaceData(enabled: boolean) {
  const [chats, setChats] = useState<ChatMeta[]>([]);
  const [projects, setProjects] = useState<ProjectMeta[]>([]);

  const refreshChats = useCallback(async () => {}, []);
  const refreshProjects = useCallback(async () => {}, []);

  useEffect(() => {
    if (!enabled) {
      setChatsIfChanged([]);
      setProjectsIfChanged([]);
      return;
    }

    return workspaceApi.subscribe(applyWorkspaceMessage);
  }, [enabled]);

  function applyWorkspaceMessage(message: WorkspaceMessage) {
    switch (message.type) {
      case "workspace.snapshot":
        setChatsIfChanged(message.chats);
        setProjectsIfChanged(message.projects);
        break;
      case "chat.upsert":
        setChats((current) => nextChats(upsertById(current, message.chat), current));
        break;
      case "chat.delete":
        setChats((current) => nextChats(current.filter((chat) => chat.id !== message.id), current));
        break;
      case "project.upsert":
        setProjects((current) => nextProjects(upsertById(current, message.project), current));
        break;
      case "project.delete":
        setProjects((current) => nextProjects(current.filter((project) => project.id !== message.id), current));
        break;
    }
  }

  function setChatsIfChanged(next: ChatMeta[]) {
    setChats((current) => nextChats(next, current));
  }

  function setProjectsIfChanged(next: ProjectMeta[]) {
    setProjects((current) => nextProjects(next, current));
  }

  return {
    chats,
    projects,
    refreshChats,
    refreshProjects,
  };
}

function upsertById<T extends { id: string }>(items: T[], item: T): T[] {
  const index = items.findIndex((candidate) => candidate.id === item.id);
  if (index < 0) return [...items, item];
  const next = items.slice();
  next[index] = item;
  return next;
}

function nextChats(next: ChatMeta[], current?: ChatMeta[]): ChatMeta[] {
  const sorted = next.slice().sort((a, b) => b.lastMessageAt - a.lastMessageAt);
  return current && sameChats(current, sorted) ? current : sorted;
}

function nextProjects(next: ProjectMeta[], current?: ProjectMeta[]): ProjectMeta[] {
  const sorted = next.slice().sort(compareProjects);
  return current && sameProjects(current, sorted) ? current : sorted;
}

function compareProjects(a: ProjectMeta, b: ProjectMeta): number {
  const left = projectOrder(a);
  const right = projectOrder(b);
  if (left !== right) return right - left;
  return b.createdAt - a.createdAt;
}

function projectOrder(project: ProjectMeta): number {
  return project.order || project.createdAt || 0;
}

function sameChats(a: ChatMeta[], b: ChatMeta[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    const left = a[i];
    const right = b[i];
    if (
      left.id !== right.id ||
      left.title !== right.title ||
      left.provider !== right.provider ||
      left.claudeSessionId !== right.claudeSessionId ||
      left.codexSessionId !== right.codexSessionId ||
      left.tmuxSession !== right.tmuxSession ||
      left.cwd !== right.cwd ||
      left.createdAt !== right.createdAt ||
      left.lastMessageAt !== right.lastMessageAt ||
      left.lastReadAt !== right.lastReadAt ||
      left.running !== right.running ||
      left.model !== right.model ||
      left.mode !== right.mode ||
      left.reasoningEffort !== right.reasoningEffort ||
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
      left.order !== right.order ||
      left.errorMsg !== right.errorMsg ||
      left.createdAt !== right.createdAt ||
      left.updatedAt !== right.updatedAt
    ) {
      return false;
    }
  }
  return true;
}
