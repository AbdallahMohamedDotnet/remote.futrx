import { useEffect, useState } from "preact/hooks";
import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";

const sidebarCollapsedKey = "remote.futrx.sidebarCollapsed";

export function useSidebarState(
  open: boolean,
  onClose: () => void,
  projects: ProjectMeta[],
  chats: ChatMeta[]
) {
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => readSidebarCollapsed());

  useEffect(() => {
    if (!open) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  useEffect(() => {
    setCollapsed((current) => {
      const next: Record<string, boolean> = {};
      for (const project of projects) {
        next[project.id] = !projectHasUnreadOrRunningChat(project.id, chats);
      }
      return sameCollapsedState(current, next) ? current : next;
    });
  }, [projects, chats]);

  useEffect(() => {
    try {
      localStorage.setItem(sidebarCollapsedKey, sidebarCollapsed ? "true" : "false");
    } catch {}
  }, [sidebarCollapsed]);

  function toggleCollapsed(id: string) {
    setCollapsed((current) => ({ ...current, [id]: !current[id] }));
  }

  function toggleSidebarCollapsed() {
    setSidebarCollapsed((current) => !current);
  }

  return {
    query,
    setQuery,
    collapsed,
    toggleCollapsed,
    sidebarCollapsed,
    toggleSidebarCollapsed,
  };
}

function projectHasUnreadOrRunningChat(projectId: string, chats: ChatMeta[]): boolean {
  return chats.some((chat) =>
    chat.projectId === projectId && (chat.running || isUnread(chat))
  );
}

function isUnread(chat: ChatMeta): boolean {
  return (chat.lastMessageAt || 0) > (chat.lastReadAt || 0);
}

function sameCollapsedState(a: Record<string, boolean>, b: Record<string, boolean>): boolean {
  const aKeys = Object.keys(a);
  const bKeys = Object.keys(b);
  if (aKeys.length !== bKeys.length) return false;
  return bKeys.every((key) => a[key] === b[key]);
}

function readSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(sidebarCollapsedKey) === "true";
  } catch {
    return false;
  }
}
