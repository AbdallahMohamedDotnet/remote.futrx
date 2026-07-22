import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";

const SIDEBAR_COLLAPSED_KEY = "remote.futrx.sidebarCollapsed";

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

class WorkspaceSidebarState {
  activeChat(chats: ChatMeta[], activeChatId: string | null): ChatMeta | null {
    return activeChatId ? chats.find((chat) => chat.id === activeChatId) ?? null : null;
  }

  initialChatId(
    gateOpen: boolean,
    activeChatId: string | null,
    chats: ChatMeta[]
  ): string | null {
    if (!gateOpen || activeChatId !== null || chats.length === 0) return null;
    return chats[0].id;
  }

  isActiveChatMissing(chats: ChatMeta[], activeChatId: string | null): boolean {
    return !!activeChatId && !chats.some((chat) => chat.id === activeChatId);
  }

  model(chats: ChatMeta[], projects: ProjectMeta[], rawQuery: string): WorkspaceSidebarModel {
    const query = rawQuery.trim().toLowerCase();
    const buckets = this.bucketChatsByProject(chats);
    const sortedProjects = [...projects].sort((left, right) => this.compareProjects(left, right));

    const visibleProjects = sortedProjects
      .map((project) => {
        const projectChats = buckets.byProject.get(project.id) ?? [];
        const projectMatches = this.matchesProject(project, query);
        const filteredChats = query
          ? projectChats.filter((chat) => projectMatches || this.matchesChat(chat, query))
          : projectChats;
        return { project, chats: projectChats, filteredChats };
      })
      .filter(
        (node) =>
          !query || this.matchesProject(node.project, query) || node.filteredChats.length > 0
      );

    const visibleLooseChats = buckets.loose.filter((chat) => this.matchesChat(chat, query));

    return {
      visibleProjects,
      visibleLooseChats,
      totalChats: chats.length,
      totalProjects: projects.length,
      hasMatches: visibleProjects.length > 0 || visibleLooseChats.length > 0,
      query,
    };
  }

  collapsedProjects(projects: ProjectMeta[], chats: ChatMeta[]): Record<string, boolean> {
    const collapsed: Record<string, boolean> = {};
    for (const project of projects) {
      collapsed[project.id] = !this.projectHasUnreadChat(project.id, chats);
    }
    return collapsed;
  }

  hasSameCollapsedProjects(
    current: Record<string, boolean>,
    next: Record<string, boolean>
  ): boolean {
    const currentKeys = Object.keys(current);
    const nextKeys = Object.keys(next);
    if (currentKeys.length !== nextKeys.length) return false;
    return nextKeys.every((key) => current[key] === next[key]);
  }

  readCollapsed(): boolean {
    try {
      return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "true";
    } catch {
      return false;
    }
  }

  writeCollapsed(collapsed: boolean): void {
    try {
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? "true" : "false");
    } catch {}
  }

  private bucketChatsByProject(chats: ChatMeta[]): ChatBuckets {
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
      projectChats.sort((left, right) => right.lastMessageAt - left.lastMessageAt);
    }
    loose.sort((left, right) => right.lastMessageAt - left.lastMessageAt);

    return { byProject, loose };
  }

  private projectHasUnreadChat(projectId: string, chats: ChatMeta[]): boolean {
    return chats.some(
      (chat) =>
        chat.projectId === projectId && (chat.lastMessageAt || 0) > (chat.lastReadAt || 0)
    );
  }

  private matchesChat(chat: ChatMeta, query: string): boolean {
    if (!query) return true;
    return `${chat.title} ${chat.cwd ?? ""} ${chat.model ?? ""}`
      .toLowerCase()
      .includes(query);
  }

  private matchesProject(project: ProjectMeta, query: string): boolean {
    if (!query) return true;
    return `${project.name} ${project.slug}`.toLowerCase().includes(query);
  }

  private compareProjects(left: ProjectMeta, right: ProjectMeta): number {
    const leftOrder = left.order || left.createdAt || 0;
    const rightOrder = right.order || right.createdAt || 0;
    if (leftOrder !== rightOrder) return rightOrder - leftOrder;
    return right.updatedAt - left.updatedAt;
  }
}

export const workspaceSidebarState = new WorkspaceSidebarState();
