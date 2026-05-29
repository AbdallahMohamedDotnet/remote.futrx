import { useEffect, useState } from "preact/hooks";

export function useSidebarState(open: boolean, onClose: () => void) {
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (!open) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  function toggleCollapsed(id: string) {
    setCollapsed((current) => ({ ...current, [id]: !current[id] }));
  }

  return {
    query,
    setQuery,
    collapsed,
    toggleCollapsed,
  };
}
