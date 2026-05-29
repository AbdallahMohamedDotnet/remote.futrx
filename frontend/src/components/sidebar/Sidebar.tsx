import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";
import type { WorkspaceSidebarModel } from "../../state/workspace/selectors";
import { ChatRow } from "./ChatRow";
import { ProjectGroup } from "./ProjectGroup";
import { SidebarEmptyState, SidebarNoMatches } from "./SidebarEmptyState";
import { WorkspaceSearch } from "./WorkspaceSearch";
import { AccountFooter } from "./AccountFooter";
import { Plus, X } from "../ui/icons";

export function Sidebar({
  open,
  model,
  query,
  collapsed,
  activeChatId,
  account,
  onClose,
  onQueryChange,
  onClearQuery,
  onNewProject,
  onNewChatInProject,
  onToggleProject,
  onSelectChat,
  onDeleteChat,
  onStartProject,
  onStopProject,
  onDeleteProject,
  onOpenSettings,
}: {
  open: boolean;
  model: WorkspaceSidebarModel;
  query: string;
  collapsed: Record<string, boolean>;
  activeChatId: string | null;
  account?: { email: string; authenticated: boolean; noAuth: boolean };
  onClose: () => void;
  onQueryChange: (query: string) => void;
  onClearQuery: () => void;
  onNewProject: () => void;
  onNewChatInProject: (projectId?: string) => void;
  onToggleProject: (projectId: string) => void;
  onSelectChat: (chatId: string) => void;
  onDeleteChat: (chat: ChatMeta, event: Event) => void;
  onStartProject: (project: ProjectMeta, event: Event) => void;
  onStopProject: (project: ProjectMeta, event: Event) => void;
  onDeleteProject: (project: ProjectMeta, event: Event) => void;
  onOpenSettings?: () => void;
}) {
  return (
    <>
      <div
        class={`md:hidden fixed inset-0 z-30 bg-black/60 transition-opacity duration-200
                ${open ? "opacity-100 pointer-events-auto" : "opacity-0 pointer-events-none"}`}
        onClick={onClose}
      />
      <aside
        data-open={open ? "true" : "false"}
        class={`codex-sidebar drawer-panel mobile-sheet safe-top fixed md:static z-40 inset-y-0 left-0 w-[min(92vw,380px)] md:w-[300px]
                ${open ? "translate-x-0" : "-translate-x-full"} md:translate-x-0
                bg-[#101318] border-r border-white/10 flex flex-col shadow-2xl md:shadow-none`}
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
              class="h-10 min-w-10 rounded-md bg-accent-blue text-white grid place-items-center hover:bg-accent-blue/90 active:scale-[0.98] transition"
              aria-label="New project"
              title="New project"
            >
              <Plus class="w-5 h-5" />
            </button>
            <button
              type="button"
              onClick={onClose}
              class="md:hidden h-10 w-10 rounded-md bg-white/5 text-ink-100 grid place-items-center hover:bg-white/10 active:scale-[0.98] transition"
              aria-label="Close sidebar"
              title="Close"
            >
              <X class="w-5 h-5" />
            </button>
          </div>

          <WorkspaceSearch query={query} onQueryChange={onQueryChange} onClear={onClearQuery} />
        </header>

        <div class="px-3 py-2 flex items-center justify-between gap-2 text-[12px] text-ink-300">
          <span>
            {model.totalProjects} project{model.totalProjects === 1 ? "" : "s"}
            {" - "}
            {model.totalChats} chat{model.totalChats === 1 ? "" : "s"}
          </span>
        </div>

        <div class="flex-1 overflow-y-auto touch-scroll px-2 pb-3 space-y-2">
          {model.totalProjects === 0 && model.totalChats === 0 && (
            <SidebarEmptyState onNewProject={onNewProject} />
          )}

          {model.query && !model.hasMatches && <SidebarNoMatches />}

          {model.visibleProjects.map((node) => (
            <ProjectGroup
              key={node.project.id}
              project={node.project}
              chats={node.chats}
              visibleChats={node.filteredChats}
              activeChatId={activeChatId}
              collapsed={collapsed[node.project.id] === true}
              onToggle={() => onToggleProject(node.project.id)}
              onNewChat={() => onNewChatInProject(node.project.id)}
              onStart={(event) => onStartProject(node.project, event)}
              onStop={(event) => onStopProject(node.project, event)}
              onDelete={(event) => onDeleteProject(node.project, event)}
              onSelectChat={onSelectChat}
              onDeleteChat={onDeleteChat}
            />
          ))}

          {model.visibleLooseChats.length > 0 && (
            <div class="pt-2">
              <div class="px-3 pt-2 pb-1 text-[10.5px] uppercase tracking-wider text-ink-400 font-semibold">
                Unassigned
              </div>
              <div class="space-y-0.5">
                {model.visibleLooseChats.map((chat) => (
                  <ChatRow
                    key={chat.id}
                    chat={chat}
                    active={chat.id === activeChatId}
                    onSelect={() => onSelectChat(chat.id)}
                    onDelete={(event) => onDeleteChat(chat, event)}
                  />
                ))}
              </div>
            </div>
          )}
        </div>

        {account && !account.noAuth && account.authenticated && (
          <AccountFooter email={account.email} onOpenSettings={onOpenSettings} />
        )}
      </aside>
    </>
  );
}
