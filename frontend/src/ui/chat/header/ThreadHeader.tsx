import type { ChatMeta } from "../../../models/chat";
import { providerDisplayLabel } from "../../../config/chat";
import { Menu, MessageSquare } from "../../primitives/icons";
import { WorkspaceActions } from "./WorkspaceActions";

export function ThreadHeader({
  chat,
  streaming,
  onOpenTerminal,
  onOpenBrowser,
  onOpenHistory,
  onOpenFiles,
  onHamburger,
}: {
  chat: ChatMeta;
  streaming: boolean;
  onOpenTerminal: () => void;
  onOpenBrowser: () => void;
  onOpenHistory: () => void;
  onOpenFiles: () => void;
  onHamburger: () => void;
}) {
  return (
    <header class="codex-header top-chrome flex-none z-20 bg-[#101318] md:bg-[#101318]/95 md:backdrop-blur border-b border-white/10 px-3 md:px-4 pb-2 flex flex-col gap-2">
      <div class="flex items-center gap-2 min-h-11">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 rounded-md text-ink-100 hover:bg-white/[0.08] grid place-items-center flex-none"
          aria-label="Open chats"
          title="Chats"
        >
          <Menu class="w-5 h-5" />
        </button>

        <div class="hidden sm:grid h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 text-ink-200 place-items-center flex-none">
          <MessageSquare class="w-4 h-4" />
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 min-w-0">
            <h1 class="truncate text-[15px] md:text-base font-semibold text-ink-50">
              {chat.title || "Untitled chat"}
            </h1>
            <span
              class={`h-2 w-2 rounded-full flex-none ${streaming ? "bg-accent-green animate-pulse" : "bg-ink-400"}`}
              title={streaming ? "Streaming" : "Ready"}
            />
          </div>
          <div class="text-[12px] text-ink-300 truncate">
            {providerDisplayLabel(chat.provider)} · {streaming ? "Working" : "Ready"}
          </div>
        </div>
      </div>

      <div class="flex w-full items-center gap-2 overflow-x-auto no-scrollbar pb-0.5">
        <WorkspaceActions
          cwd={chat.cwd || "~"}
          onOpenTerminal={onOpenTerminal}
          onOpenBrowser={onOpenBrowser}
          onOpenHistory={onOpenHistory}
          onOpenFiles={onOpenFiles}
        />
      </div>
    </header>
  );
}
