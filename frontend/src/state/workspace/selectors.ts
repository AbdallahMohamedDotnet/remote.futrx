import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";

export interface ProjectSidebarNode {
  project: ProjectMeta;
  chats: ChatMeta[];
  filteredChats: ChatMeta[];
}

export interface WorkspaceSidebarModel {
  visibleProjects: ProjectSidebarNode[];
  visibleLooseChats: ChatMeta[];
  totalChats: number;
  totalProjects: number;
  hasMatches: boolean;
  query: string;
}

interface ChatBuckets {
  byProject: Map<string, ChatMeta[]>;
  loose: ChatMeta[];
}

export function selectActiveChat(chats: ChatMeta[], activeChatId: string | null): ChatMeta | null {
  return activeChatId ? chats.find((chat) => chat.id === activeChatId) ?? null : null;
}

export function shouldSelectInitialChat(
  gateOpen: boolean,
  activeChatId: string | null,
  chats: ChatMeta[]
): string | null {
  if (!gateOpen || activeChatId !== null || chats.length === 0) return null;
  return chats[0].id;
}

export function isActiveChatMissing(chats: ChatMeta[], activeChatId: string | null): boolean {
  return !!activeChatId && !chats.some((chat) => chat.id === activeChatId);
}

export function buildWorkspaceSidebarModel(
  chats: ChatMeta[],
  projects: ProjectMeta[],
  rawQuery: string
): WorkspaceSidebarModel {
  const query = rawQuery.trim().toLowerCase();
  const buckets = bucketChatsByProject(chats);
  const sortedProjects = [...projects].sort(compareProjects);

  const visibleProjects = sortedProjects
    .map((project) => {
      const projectChats = buckets.byProject.get(project.id) ?? [];
      const projectMatches = matchesProject(project, query);
      const filteredChats = query
        ? projectChats.filter((chat) => projectMatches || matchesChat(chat, query))
        : projectChats;
      return { project, chats: projectChats, filteredChats };
    })
    .filter((node) => !query || matchesProject(node.project, query) || node.filteredChats.length > 0);

  const visibleLooseChats = buckets.loose.filter((chat) => matchesChat(chat, query));

  return {
    visibleProjects,
    visibleLooseChats,
    totalChats: chats.length,
    totalProjects: projects.length,
    hasMatches: visibleProjects.length > 0 || visibleLooseChats.length > 0,
    query,
  };
}

export function modelLabel(model?: string): string {
  if (!model) return "auto";
  const lower = model.toLowerCase();
  if (lower.includes("opus")) return "opus";
  if (lower.includes("sonnet")) return "sonnet";
  if (lower.includes("haiku")) return "haiku";
  return model;
}

function bucketChatsByProject(chats: ChatMeta[]): ChatBuckets {
  const byProject = new Map<string, ChatMeta[]>();
  const loose: ChatMeta[] = [];

  for (const chat of chats) {
    if (!chat.projectId) {
      loose.push(chat);
      continue;
    }

    const projectChats = byProject.get(chat.projectId) ?? [];
    projectChats.push(chat);
    byProject.set(chat.projectId, projectChats);
  }

  for (const projectChats of byProject.values()) {
    projectChats.sort((a, b) => b.lastMessageAt - a.lastMessageAt);
  }
  loose.sort((a, b) => b.lastMessageAt - a.lastMessageAt);

  return { byProject, loose };
}

function matchesChat(chat: ChatMeta, query: string): boolean {
  if (!query) return true;
  return `${chat.title} ${chat.cwd ?? ""} ${chat.model ?? ""}`.toLowerCase().includes(query);
}

function matchesProject(project: ProjectMeta, query: string): boolean {
  if (!query) return true;
  return `${project.name} ${project.slug}`.toLowerCase().includes(query);
}

function compareProjects(a: ProjectMeta, b: ProjectMeta): number {
  const left = a.order || a.createdAt || 0;
  const right = b.order || b.createdAt || 0;
  if (left !== right) return right - left;
  return b.updatedAt - a.updatedAt;
}
