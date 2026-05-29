import { useEffect, useMemo, useState } from "preact/hooks";
import { chatsApi, projectsApi } from "../lib/api";
import { timeAgo } from "../lib/format";
import type { ChatMeta, ProjectMeta, ProjectStatus } from "../types";
import type { AuthState } from "../lib/useAuth";
import {
  ChevronDown,
  ChevronRight,
  Clock,
  Folder,
  Loader,
  LogOut,
  MessageSquare,
  Plus,
  Search,
  Settings,
  X,
} from "./icons";

interface Props {
  chats: ChatMeta[];
  projects: ProjectMeta[];
  activeChatId: string | null;
  onSelect: (id: string) => void;
  onRefreshChats: () => void;
  onRefreshProjects: () => void;
  onNewProject: () => void;
  onNewChatInProject: (projectId?: string) => void;
  onOpenSettings?: () => void;
  open: boolean;
  onClose: () => void;
  auth?: AuthState;
}

function modelLabel(m?: string): string {
  if (!m) return "auto";
  const lower = m.toLowerCase();
  if (lower.includes("opus")) return "opus";
  if (lower.includes("sonnet")) return "sonnet";
  if (lower.includes("haiku")) return "haiku";
  return m;
}

function matchesChat(c: ChatMeta, q: string): boolean {
  if (!q) return true;
  return `${c.title} ${c.cwd ?? ""} ${c.model ?? ""}`.toLowerCase().includes(q);
}

function matchesProject(p: ProjectMeta, q: string): boolean {
  if (!q) return true;
  return `${p.name} ${p.slug}`.toLowerCase().includes(q);
}

// Status dot — visual indicator of container state.
function StatusDot({ status }: { status: ProjectStatus }) {
  if (status === "provisioning") {
    return (
      <span
        class="flex-none w-2 h-2 rounded-full bg-accent-yellow animate-pulse"
        title="Provisioning"
      />
    );
  }
  const cls: Record<string, string> = {
    running: "bg-accent-green",
    stopped: "bg-ink-400",
    error: "bg-accent-red",
    missing: "bg-accent-red/60 ring-1 ring-accent-red",
    "": "bg-ink-400",
  };
  const label: Record<string, string> = {
    running: "Running",
    stopped: "Stopped",
    error: "Error",
    missing: "Missing — needs reprovision",
    "": "Unknown",
  };
  return (
    <span
      class={`flex-none w-2 h-2 rounded-full ${cls[status] ?? "bg-ink-400"}`}
      title={label[status] ?? "Unknown"}
    />
  );
}

export function ChatSidebar({
  chats,
  projects,
  activeChatId,
  onSelect,
  onRefreshChats,
  onRefreshProjects,
  onNewProject,
  onNewChatInProject,
  onOpenSettings,
  open,
  onClose,
  auth,
}: Props) {
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (!open) return;
    const h = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [open, onClose]);

  const q = query.trim().toLowerCase();

  const sortedProjects = useMemo(
    () => [...projects].sort((a, b) => b.updatedAt - a.updatedAt),
    [projects]
  );

  // Bucket chats by projectId; legacy chats with no projectId go into "loose".
  const chatsByProject = useMemo(() => {
    const m = new Map<string, ChatMeta[]>();
    const loose: ChatMeta[] = [];
    for (const c of chats) {
      if (c.projectId) {
        const arr = m.get(c.projectId) ?? [];
        arr.push(c);
        m.set(c.projectId, arr);
      } else {
        loose.push(c);
      }
    }
    // Sort each bucket by recency.
    for (const arr of m.values()) arr.sort((a, b) => b.lastMessageAt - a.lastMessageAt);
    loose.sort((a, b) => b.lastMessageAt - a.lastMessageAt);
    return { byProject: m, loose };
  }, [chats]);

  // A project shows up if it matches the query OR any of its chats do.
  const visibleProjects = useMemo(() => {
    if (!q) return sortedProjects;
    return sortedProjects.filter((p) => {
      if (matchesProject(p, q)) return true;
      const pcs = chatsByProject.byProject.get(p.id) ?? [];
      return pcs.some((c) => matchesChat(c, q));
    });
  }, [sortedProjects, chatsByProject, q]);

  const visibleLoose = useMemo(
    () => chatsByProject.loose.filter((c) => matchesChat(c, q)),
    [chatsByProject.loose, q]
  );

  async function deleteChat(c: ChatMeta, ev: Event) {
    ev.stopPropagation();
    if (!confirm(`Delete chat "${c.title}"? This removes its history.`)) return;
    try {
      await chatsApi.delete(c.id);
      onRefreshChats();
    } catch (e) {
      alert("delete failed: " + (e as Error).message);
    }
  }

  async function deleteProject(p: ProjectMeta, ev: Event) {
    ev.stopPropagation();
    const n = chatsByProject.byProject.get(p.id)?.length ?? 0;
    const msg = n > 0
      ? `Delete project "${p.name}"? This will destroy the container and remove ${n} chat${n === 1 ? "" : "s"} inside it.`
      : `Delete project "${p.name}"? This will destroy its container.`;
    if (!confirm(msg)) return;
    try {
      await projectsApi.delete(p.id);
      onRefreshProjects();
      onRefreshChats();
    } catch (e) {
      alert("delete failed: " + (e as Error).message);
    }
  }

  async function startProject(p: ProjectMeta, ev: Event) {
    ev.stopPropagation();
    try {
      await projectsApi.start(p.id);
      onRefreshProjects();
    } catch (e) {
      alert("start failed: " + (e as Error).message);
    }
  }

  async function stopProject(p: ProjectMeta, ev: Event) {
    ev.stopPropagation();
    try {
      await projectsApi.stop(p.id);
      onRefreshProjects();
    } catch (e) {
      alert("stop failed: " + (e as Error).message);
    }
  }

  function toggle(id: string) {
    setCollapsed((c) => ({ ...c, [id]: !c[id] }));
  }

  const totalChats = chats.length;
  const totalProjects = projects.length;

  return (
    <>
      <div
        class={`md:hidden fixed inset-0 z-30 bg-black/60 transition-opacity duration-200
                ${open ? "opacity-100 pointer-events-auto" : "opacity-0 pointer-events-none"}`}
        onClick={onClose}
      />
      <aside
        data-open={open ? "true" : "false"}
        class="codex-sidebar drawer-panel mobile-sheet safe-top fixed md:static z-40 inset-y-0 left-0 w-[min(92vw,380px)] md:w-[300px]
                bg-[#101318] border-r border-white/10 flex flex-col shadow-2xl md:shadow-none"
      >
        <header class="px-3 pt-3 pb-2 border-b border-white/10">
          <div class="flex items-center gap-2 min-h-11">
            <div class="flex-1 min-w-0">
              <div class="text-[11px] text-ink-300">Workspace</div>
              <div class="text-[15px] font-semibold text-ink-50 truncate">Projects</div>
            </div>
            <button
              type="button"
              onClick={onNewProject}
              class="h-10 min-w-10 rounded-md bg-accent-blue text-white grid place-items-center
                     hover:bg-accent-blue/90 active:scale-[0.98] transition"
              aria-label="New project"
              title="New project"
            >
              <Plus class="w-5 h-5" />
            </button>
            <button
              type="button"
              onClick={onClose}
              class="md:hidden h-10 w-10 rounded-md bg-white/5 text-ink-100 grid place-items-center
                     hover:bg-white/10 active:scale-[0.98] transition"
              aria-label="Close sidebar"
              title="Close"
            >
              <X class="w-5 h-5" />
            </button>
          </div>

          <label class="mt-3 flex items-center gap-2 h-10 rounded-md bg-[#0b0d11] border border-white/10 px-3
                        focus-within:border-accent-blue/70 transition-colors">
            <Search class="w-4 h-4 text-ink-300 flex-none" />
            <input
              value={query}
              onInput={(e) => setQuery((e.currentTarget as HTMLInputElement).value)}
              placeholder="Search projects and chats"
              class="min-w-0 flex-1 bg-transparent text-[14px] text-ink-100 placeholder:text-ink-300
                     focus:outline-none"
              autocomplete="off"
              spellcheck={false}
            />
            {query && (
              <button
                type="button"
                onClick={() => setQuery("")}
                class="w-7 h-7 grid place-items-center rounded text-ink-300 hover:bg-white/10 hover:text-ink-100"
                aria-label="Clear search"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            )}
          </label>
        </header>

        <div class="px-3 py-2 flex items-center justify-between gap-2 text-[12px] text-ink-300">
          <span>
            {totalProjects} project{totalProjects === 1 ? "" : "s"}
            {" · "}
            {totalChats} chat{totalChats === 1 ? "" : "s"}
          </span>
        </div>

        <div class="flex-1 overflow-y-auto touch-scroll px-2 pb-3 space-y-2">
          {/* Empty state */}
          {totalProjects === 0 && totalChats === 0 && (
            <div class="mx-2 rounded-lg border border-dashed border-white/[0.12] bg-white/[0.03] text-center text-ink-300 text-sm py-8 px-4">
              <Folder class="w-8 h-8 mx-auto mb-3 opacity-50" />
              <div class="text-ink-100 font-medium">No projects yet</div>
              <div class="text-[12px] mt-1.5 leading-relaxed">
                Each project gets its own sandboxed container. Claude runs inside.
              </div>
              <button
                type="button"
                onClick={onNewProject}
                class="mt-4 inline-flex items-center gap-1.5 h-9 px-3 rounded-md
                       bg-white/[0.08] hover:bg-white/[0.12] text-ink-100 text-sm"
              >
                <Plus class="w-4 h-4" /> New project
              </button>
            </div>
          )}

          {q && visibleProjects.length === 0 && visibleLoose.length === 0 && (
            <div class="mx-2 rounded-lg border border-dashed border-white/[0.12] bg-white/[0.03] text-center text-ink-300 text-sm py-6 px-4">
              <Search class="w-6 h-6 mx-auto mb-2 opacity-50" />
              No matches
            </div>
          )}

          {/* Projects */}
          {visibleProjects.map((p) => {
            const projChats = chatsByProject.byProject.get(p.id) ?? [];
            const filteredChats = q
              ? projChats.filter((c) => matchesProject(p, q) || matchesChat(c, q))
              : projChats;
            const isCollapsed = collapsed[p.id] === true;
            const provisioning = p.status === "provisioning";
            const stopped = p.status === "stopped";
            return (
              <div key={p.id} class="rounded-lg">
                <div class="group flex items-stretch gap-0.5 rounded-md hover:bg-white/[0.04]">
                  <button
                    type="button"
                    onClick={() => toggle(p.id)}
                    class="w-7 grid place-items-center text-ink-300 hover:text-ink-100"
                    aria-label={isCollapsed ? "Expand" : "Collapse"}
                    title={isCollapsed ? "Expand" : "Collapse"}
                  >
                    {isCollapsed
                      ? <ChevronRight class="w-4 h-4" />
                      : <ChevronDown class="w-4 h-4" />}
                  </button>
                  <div class="flex-1 min-w-0 py-2 pr-1 flex items-center gap-2">
                    <StatusDot status={p.status} />
                    <Folder class="w-4 h-4 flex-none text-ink-300" />
                    <div class="flex-1 min-w-0">
                      <div class="text-[13.5px] leading-tight truncate font-medium text-ink-100">
                        {p.name}
                      </div>
                      <div class="text-[11px] text-ink-400 truncate font-mono">
                        {p.slug}
                        {projChats.length > 0 && (
                          <span class="ml-1.5 text-ink-300">
                            · {projChats.length} chat{projChats.length === 1 ? "" : "s"}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); onNewChatInProject(p.id); }}
                    disabled={provisioning}
                    class="w-9 grid place-items-center text-ink-300 hover:text-ink-50 hover:bg-white/[0.08]
                           rounded disabled:opacity-40 disabled:cursor-not-allowed"
                    aria-label="New chat in project"
                    title={provisioning ? "Project is still provisioning" : "New chat in this project"}
                  >
                    {provisioning ? <Loader class="w-3.5 h-3.5 animate-spin" /> : <Plus class="w-4 h-4" />}
                  </button>
                  <button
                    type="button"
                    onClick={stopped
                      ? (e) => startProject(p, e)
                      : (e) => stopProject(p, e)}
                    disabled={provisioning || p.status === "" || p.status === "missing"}
                    class="hidden md:grid w-9 place-items-center text-[10px] uppercase tracking-wide
                           text-ink-300 hover:text-ink-50 hover:bg-white/[0.08] rounded
                           opacity-0 group-hover:opacity-100 transition-opacity
                           disabled:opacity-0"
                    aria-label={stopped ? "Start project" : "Stop project"}
                    title={stopped ? "Start container" : "Stop container"}
                  >
                    {stopped ? "Start" : "Stop"}
                  </button>
                  <button
                    type="button"
                    onClick={(e) => deleteProject(p, e)}
                    class="w-9 grid place-items-center text-ink-300 hover:text-accent-red hover:bg-accent-red/10
                           rounded opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity"
                    aria-label={`Delete ${p.name}`}
                    title="Delete project"
                  >
                    <X class="w-4 h-4" />
                  </button>
                </div>

                {p.errorMsg && (
                  <div class="ml-7 mt-1 mr-1 text-[11px] text-accent-red bg-accent-red/10 border border-accent-red/30
                              rounded px-2 py-1.5 break-words font-mono">
                    {p.errorMsg}
                  </div>
                )}

                {!isCollapsed && (
                  <div class="ml-5 pl-2 mt-1 space-y-0.5 border-l border-white/[0.08]">
                    {filteredChats.length === 0 ? (
                      <button
                        type="button"
                        onClick={() => onNewChatInProject(p.id)}
                        disabled={provisioning}
                        class="ml-2 mt-1 mb-2 inline-flex items-center gap-1.5 h-7 px-2 rounded
                               text-[12px] text-ink-300 hover:text-ink-100 hover:bg-white/[0.06]
                               disabled:opacity-40 disabled:cursor-not-allowed"
                      >
                        <Plus class="w-3.5 h-3.5" /> New chat
                      </button>
                    ) : (
                      filteredChats.map((c) => renderChatRow(c, activeChatId, onSelect, deleteChat))
                    )}
                  </div>
                )}
              </div>
            );
          })}

          {/* Loose / legacy chats */}
          {visibleLoose.length > 0 && (
            <div class="pt-2">
              <div class="px-3 pt-2 pb-1 text-[10.5px] uppercase tracking-wider text-ink-400 font-semibold">
                Unassigned
              </div>
              <div class="space-y-0.5">
                {visibleLoose.map((c) => renderChatRow(c, activeChatId, onSelect, deleteChat))}
              </div>
            </div>
          )}
        </div>

        {auth && !auth.noAuth && auth.authenticated && (
          <footer class="safe-bottom-control border-t border-white/10 px-3 pt-3 flex items-center gap-2 text-sm bg-[#0d1015]">
            <div class="w-9 h-9 rounded-md bg-accent-green/15 text-accent-green
                        grid place-items-center font-semibold flex-none">
              {(auth.email[0] || "?").toUpperCase()}
            </div>
            <span class="flex-1 min-w-0 truncate text-ink-200" title={auth.email}>{auth.email}</span>
            {onOpenSettings && (
              <button
                type="button"
                onClick={onOpenSettings}
                class="h-9 w-9 rounded-md text-ink-300 hover:text-ink-50 hover:bg-white/[0.08]
                       grid place-items-center flex-none"
                title="Settings"
                aria-label="Settings"
              >
                <Settings class="w-4 h-4" />
              </button>
            )}
            <a
              href="/auth/logout"
              class="h-9 w-9 rounded-md text-ink-300 hover:text-accent-red hover:bg-accent-red/10
                     grid place-items-center flex-none"
              title="Sign out"
              aria-label="Sign out"
            >
              <LogOut class="w-4 h-4" />
            </a>
          </footer>
        )}
      </aside>
    </>
  );
}

function renderChatRow(
  c: ChatMeta,
  activeChatId: string | null,
  onSelect: (id: string) => void,
  deleteChat: (c: ChatMeta, ev: Event) => void
) {
  const active = c.id === activeChatId;
  return (
    <div
      key={c.id}
      class={`group flex items-stretch gap-0.5 rounded transition-colors
              ${active
                ? "bg-accent-blue/[0.14] border border-accent-blue/[0.32]"
                : "border border-transparent hover:bg-white/[0.04]"}`}
    >
      <button
        type="button"
        onClick={() => onSelect(c.id)}
        class="flex-1 min-w-0 text-left px-2.5 py-2"
      >
        <div class="flex items-start gap-2">
          <MessageSquare
            class={`mt-0.5 w-3.5 h-3.5 flex-none ${active ? "text-accent-blue" : "text-ink-400"}`}
          />
          <div class="flex-1 min-w-0">
            <div class={`text-[13px] leading-snug truncate ${active ? "text-ink-50 font-medium" : "text-ink-100"}`}>
              {c.title || "Untitled"}
            </div>
            <div class="mt-0.5 flex items-center gap-1.5 text-[11px] text-ink-400">
              <span class={`px-1 py-0.5 rounded bg-white/[0.06] ${active ? "text-accent-blue" : ""}`}>
                {modelLabel(c.model)}
              </span>
              <Clock class="w-3 h-3 flex-none" />
              <span class="truncate">{timeAgo(c.lastMessageAt)}</span>
            </div>
          </div>
        </div>
      </button>
      <button
        type="button"
        onClick={(e) => deleteChat(c, e)}
        class="w-8 grid place-items-center rounded-r text-ink-300 hover:text-accent-red hover:bg-accent-red/10
               opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity"
        aria-label={`Delete ${c.title || "chat"}`}
        title="Delete chat"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  );
}
