import type { ChatMeta } from "../../models/chat";
import { formatModelShortLabel, timeAgo } from "../../lib/format";
import { Clock, MessageSquare, X } from "../ui/icons";

export function ChatRow({
  chat,
  active,
  onSelect,
  onDelete,
}: {
  chat: ChatMeta;
  active: boolean;
  onSelect: () => void;
  onDelete: (event: Event) => void;
}) {
  return (
    <div
      class={`group flex items-stretch gap-0.5 rounded transition-colors
              ${active
                ? "bg-accent-blue/[0.14] border border-accent-blue/[0.32]"
                : "border border-transparent hover:bg-white/[0.04]"}`}
    >
      <button
        type="button"
        onClick={onSelect}
        class="flex-1 min-w-0 text-left px-2.5 py-2"
      >
        <div class="flex items-start gap-2">
          <MessageSquare
            class={`mt-0.5 w-3.5 h-3.5 flex-none ${active ? "text-accent-blue" : "text-ink-400"}`}
          />
          <div class="flex-1 min-w-0">
            <div class={`text-[13px] leading-snug truncate ${active ? "text-ink-50 font-medium" : "text-ink-100"}`}>
              {chat.title || "Untitled"}
            </div>
            <div class="mt-0.5 flex items-center gap-1.5 text-[11px] text-ink-400">
              <span class={`px-1 py-0.5 rounded bg-white/[0.06] ${active ? "text-accent-blue" : ""}`}>
                {formatModelShortLabel(chat.model)}
              </span>
              <Clock class="w-3 h-3 flex-none" />
              <span class="truncate">{timeAgo(chat.lastMessageAt)}</span>
            </div>
          </div>
        </div>
      </button>
      <button
        type="button"
        onClick={onDelete}
        class="w-8 grid place-items-center rounded-r text-ink-300 hover:text-accent-red hover:bg-accent-red/10
               opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity"
        aria-label={`Delete ${chat.title || "chat"}`}
        title="Delete chat"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  );
}
