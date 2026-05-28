import { useEffect } from "preact/hooks";
import { chatsApi } from "../lib/api";
import { shortenPath, timeAgo } from "../lib/format";
import type { ChatMeta } from "../types";
import type { AuthState } from "../lib/useAuth";
import { Plus, X, MessageSquare } from "./icons";

interface Props {
  chats: ChatMeta[];
  activeChatId: string | null;
  onSelect: (id: string) => void;
  onRefresh: () => void;
  open: boolean;
  onClose: () => void;
  auth?: AuthState;
}

function modelLabel(m?: string): string {
  if (!m) return "default";
  const lower = m.toLowerCase();
  if (lower.includes("opus"))   return "opus";
  if (lower.includes("sonnet")) return "sonnet";
  if (lower.includes("haiku"))  return "haiku";
  return m;
}

export function ChatSidebar({ chats, activeChatId, onSelect, onRefresh, open, onClose, auth }: Props) {
  useEffect(() => {
    if (!open) return;
    const h = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [open, onClose]);

  async function newChat() {
    try {
      const c = await chatsApi.create({});
      onRefresh();
      onSelect(c.id);
    } catch (e) {
      alert("create failed: " + (e as Error).message);
    }
  }

  async function deleteChat(c: ChatMeta, ev: Event) {
    ev.stopPropagation();
    if (!confirm(`Delete chat "${c.title}"? This removes its history.`)) return;
    try {
      await chatsApi.delete(c.id);
      onRefresh();
    } catch (e) {
      alert("delete failed: " + (e as Error).message);
    }
  }

  const sorted = [...chats].sort((a, b) => b.lastMessageAt - a.lastMessageAt);

  return (
    <>
      <div
        class={`md:hidden fixed inset-0 z-10 bg-black/55 transition-opacity
                ${open ? "opacity-100 pointer-events-auto" : "opacity-0 pointer-events-none"}`}
        onClick={onClose}
      />
      <aside
        class={`fixed md:static z-20 inset-y-0 left-0 w-[78%] max-w-[290px] md:w-64 md:max-w-none
                bg-ink-700 border-r border-ink-500 flex flex-col
                transition-transform duration-200 ease-out
                ${open ? "translate-x-0" : "-translate-x-full"} md:translate-x-0`}
      >
        <header class="flex items-center justify-between px-3 pt-4 pb-2">
          <span class="text-xs uppercase tracking-wider text-ink-200 font-semibold">
            Chats
          </span>
          <button
            type="button"
            onClick={newChat}
            class="flex items-center gap-1 bg-accent-blue/90 hover:bg-accent-blue text-white text-xs font-medium px-2.5 py-1 rounded"
            aria-label="New chat"
          >
            <Plus class="w-3 h-3" /> New
          </button>
        </header>
        <div class="flex-1 overflow-y-auto px-2 pb-3 scrollbar-thin space-y-0.5">
          {sorted.length === 0 && (
            <div class="text-center text-ink-300 text-xs py-8 px-3 leading-relaxed">
              <MessageSquare class="w-8 h-8 mx-auto mb-2 opacity-40" />
              No chats yet.<br/>
              <button
                type="button"
                onClick={newChat}
                class="mt-3 inline-flex items-center gap-1 text-accent-blue hover:underline"
              >
                <Plus class="w-3 h-3" /> Start one
              </button>
            </div>
          )}
          {sorted.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => onSelect(c.id)}
              class={`group w-full flex items-start gap-2 px-2.5 py-2 rounded text-left
                      ${c.id === activeChatId
                        ? "bg-accent-blue/15 text-accent-blue"
                        : "text-ink-100 hover:bg-ink-600"}`}
            >
              <div class="flex-1 min-w-0">
                <div class="text-[13px] leading-snug truncate font-medium">
                  {c.title || "Untitled"}
                </div>
                <div class="text-[10.5px] text-ink-300 truncate mt-0.5 flex items-center gap-1.5">
                  <span>{modelLabel(c.model)}</span>
                  <span>·</span>
                  <span>{timeAgo(c.lastMessageAt)}</span>
                </div>
                {c.cwd && (
                  <div class="text-[10.5px] text-ink-300 truncate font-mono mt-0.5">
                    {shortenPath(c.cwd)}
                  </div>
                )}
              </div>
              <span
                role="button"
                tabIndex={0}
                onClick={(e) => deleteChat(c, e)}
                class={`opacity-0 group-hover:opacity-100 ${c.id === activeChatId ? "opacity-100" : ""}
                        text-ink-200 hover:text-accent-red rounded p-0.5 flex-none`}
                aria-label={`Delete ${c.title}`}
              >
                <X class="w-3.5 h-3.5" />
              </span>
            </button>
          ))}
        </div>
        {auth && !auth.noAuth && auth.authenticated && (
          <footer class="border-t border-ink-500 px-3 py-2 flex items-center gap-2 text-xs">
            <div class="w-6 h-6 rounded-full bg-accent-blue/20 text-accent-blue
                        grid place-items-center font-semibold text-[11px] flex-none">
              {(auth.email[0] || "?").toUpperCase()}
            </div>
            <span class="flex-1 truncate text-ink-200" title={auth.email}>{auth.email}</span>
            <a
              href="/auth/logout"
              class="text-ink-300 hover:text-accent-red text-[11px] flex-none"
              title="Sign out"
            >
              Sign out
            </a>
          </footer>
        )}
      </aside>
    </>
  );
}
