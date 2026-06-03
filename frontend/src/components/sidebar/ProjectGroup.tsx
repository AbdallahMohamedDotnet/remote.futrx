import type { ChatMeta } from "../../models/chat";
import type { ProjectMeta } from "../../models/project";
import { ChevronDown, ChevronRight, Loader, Plus, Settings, X } from "../ui/icons";
import { ChatRow } from "./ChatRow";
import { ProjectStatusDot } from "./ProjectStatusDot";

export function ProjectGroup({
  project,
  chats,
  visibleChats,
  activeChatId,
  collapsed,
  onToggle,
  onNewChat,
  onStart,
  onStop,
  onDelete,
  onOpenContainer,
  onSelectChat,
  onDeleteChat,
}: {
  project: ProjectMeta;
  chats: ChatMeta[];
  visibleChats: ChatMeta[];
  activeChatId: string | null;
  collapsed: boolean;
  onToggle: () => void;
  onNewChat: () => void;
  onStart: (event: Event) => void;
  onStop: (event: Event) => void;
  onDelete: (event: Event) => void;
  onOpenContainer: () => void;
  onSelectChat: (chatId: string) => void;
  onDeleteChat: (chat: ChatMeta, event: Event) => void;
}) {
  const provisioning = project.status === "provisioning";
  const stopped = project.status === "stopped";

  return (
    <div class="min-h-0 rounded-lg">
      <div class="group flex items-stretch gap-0.5 rounded-md hover:bg-white/[0.04]">
        <button
          type="button"
          onClick={onToggle}
          class="w-7 grid place-items-center text-ink-300 hover:text-ink-100"
          aria-label={collapsed ? "Expand" : "Collapse"}
          title={collapsed ? "Expand" : "Collapse"}
        >
          {collapsed ? <ChevronRight class="w-4 h-4" /> : <ChevronDown class="w-4 h-4" />}
        </button>
        <div class="flex-1 min-w-0 py-2 pr-1 flex items-center gap-2">
          <ProjectStatusDot status={project.status} />
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              onOpenContainer();
            }}
            class="w-6 h-6 -ml-1 rounded grid place-items-center text-ink-300 hover:text-ink-50 hover:bg-white/[0.08]"
            aria-label={`Open container info for ${project.name}`}
            title="Container info"
          >
            <Settings class="w-4 h-4" />
          </button>
          <div class="flex-1 min-w-0">
            <div class="text-[13.5px] leading-tight truncate font-medium text-ink-100">
              {project.name}
            </div>
            <div class="text-[11px] text-ink-400 truncate font-mono">
              {project.slug}
              {chats.length > 0 && (
                <span class="ml-1.5 text-ink-300">
                  - {chats.length} chat{chats.length === 1 ? "" : "s"}
                </span>
              )}
            </div>
          </div>
        </div>
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            onNewChat();
          }}
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
          onClick={stopped ? onStart : onStop}
          disabled={provisioning || project.status === "" || project.status === "missing"}
          class="hidden md:grid w-9 place-items-center text-[10px] uppercase tracking-wide
                 text-ink-300 hover:text-ink-50 hover:bg-white/[0.08] rounded
                 opacity-0 group-hover:opacity-100 transition-opacity disabled:opacity-0"
          aria-label={stopped ? "Start project" : "Stop project"}
          title={stopped ? "Start container" : "Stop container"}
        >
          {stopped ? "Start" : "Stop"}
        </button>
        <button
          type="button"
          onClick={onDelete}
          class="w-9 grid place-items-center text-ink-300 hover:text-accent-red hover:bg-accent-red/10
                 rounded opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity"
          aria-label={`Delete ${project.name}`}
          title="Delete project"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      {project.errorMsg && (
        <div class="ml-7 mt-1 mr-1 text-[11px] text-accent-red bg-accent-red/10 border border-accent-red/30 rounded px-2 py-1.5 break-words font-mono">
          {project.errorMsg}
        </div>
      )}

      {!collapsed && (
        <div class="sidebar-project-chat-list ml-5 pl-2 pr-1 mt-1 space-y-0.5 border-l border-white/[0.08] overflow-y-auto touch-scroll scrollbar-thin">
          {visibleChats.length === 0 ? (
            <button
              type="button"
              onClick={onNewChat}
              disabled={provisioning}
              class="ml-2 mt-1 mb-2 inline-flex items-center gap-1.5 h-7 px-2 rounded
                     text-[12px] text-ink-300 hover:text-ink-100 hover:bg-white/[0.06]
                     disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Plus class="w-3.5 h-3.5" /> New chat
            </button>
          ) : (
            visibleChats.map((chat) => (
              <ChatRow
                key={chat.id}
                chat={chat}
                active={chat.id === activeChatId}
                onSelect={() => onSelectChat(chat.id)}
                onDelete={(event) => onDeleteChat(chat, event)}
              />
            ))
          )}
        </div>
      )}
    </div>
  );
}
