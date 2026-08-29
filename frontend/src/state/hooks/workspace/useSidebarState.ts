import { useEffect, useState } from "preact/hooks";
import type { ChatMeta } from "../../../models/chat";
import type { ProjectMeta } from "../../../models/project";
import { workspaceSidebarService } from "../../../services/workspace/workspaceSidebarService.ts";

export function useSidebarState(
  open: boolean,
  onClose: () => void,
  projects: ProjectMeta[],
  chats: ChatMeta[]
) {
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(() =>
    workspaceSidebarService.readCollapsedProjects()
  );
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() =>
    workspaceSidebarService.readCollapsed()
  );

  useEffect(() => {
    if (!open) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  useEffect(() => {
    // Nothing to seed before projects load, and pruning against an empty list
    // would drop what the last session remembered.
    if (projects.length === 0) return;
    setCollapsed((current) => {
      const next = workspaceSidebarService.collapsedProjects(projects, chats, current);
      return workspaceSidebarService.hasSameCollapsedProjects(current, next) ? current : next;
    });
  }, [projects, chats]);

  useEffect(() => {
    workspaceSidebarService.writeCollapsedProjects(collapsed);
  }, [collapsed]);

  useEffect(() => {
    workspaceSidebarService.writeCollapsed(sidebarCollapsed);
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
