import { useEffect, useState } from "preact/hooks";

const sidebarCollapsedKey = "remote.futrx.sidebarCollapsed";

export function useSidebarState(open: boolean, onClose: () => void) {
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

function readSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(sidebarCollapsedKey) === "true";
  } catch {
    return false;
  }
}
