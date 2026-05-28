import { useEffect, useMemo, useState } from "preact/hooks";
import { chatsApi } from "../lib/api";
import { shortenPath, timeAgo } from "../lib/format";
import type { ChatMeta } from "../types";
import type { AuthState } from "../lib/useAuth";
import { Clock, Folder, LogOut, MessageSquare, Plus, Search, X } from "./icons";

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
  if (!m) return "auto";
  const lower = m.toLowerCase();
  if (lower.includes("opus")) return "opus";
  if (lower.includes("sonnet")) return "sonnet";
  if (lower.includes("haiku")) return "haiku";
  return m;
}

function matchesQuery(c: ChatMeta, q: string): boolean {
  if (!q) return true;
  const haystack = `${c.title} ${c.cwd ?? ""} ${c.model ?? ""}`.toLowerCase();
  return haystack.includes(q);
}

export function ChatSidebar({ chats, activeChatId, onSelect, onRefresh, open, onClose, auth }: Props) {
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (!open) return;
    const h = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [open, onClose]);

  const sorted = useMemo(
    () => [...chats].sort((a, b) => b.lastMessageAt - a.lastMessageAt),
    [chats]
  );
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return sorted.filter((c) => matchesQuery(c, q));
  }, [sorted, query]);

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

  return (
    <>
      <div
        class={`md:hidden fixed inset-0 z-30 bg-black/55 backdrop-blur-[2px] transition-opacity duration-200
                ${open ? "opacity-100 pointer-events-auto" : "opacity-0 pointer-events-none"}`}
        onClick={onClose}
      />
      <aside
        class={`mobile-sheet safe-top fixed md:static z-40 inset-y-0 left-0 w-[min(92vw,380px)] md:w-[300px]
                bg-[#101318] border-r border-white/10 flex flex-col shadow-2xl md:shadow-none
                transition-transform duration-200 ease-out will-change-transform
                ${open ? "translate-x-0" : "-translate-x-full"} md:translate-x-0`}
      >
        <header class="px-3 pt-3 pb-2 border-b border-white/10">
          <div class="flex items-center gap-2 min-h-11">
            <div class="flex-1 min-w-0">
              <div class="text-[11px] text-ink-300">Workspace</div>
              <div class="text-[15px] font-semibold text-ink-50 truncate">Chats</div>
            </div>
            <button
              type="button"
              onClick={newChat}
              class="h-10 min-w-10 rounded-md bg-accent-blue text-white grid place-items-center
                     hover:bg-accent-blue/90 active:scale-[0.98] transition"
              aria-label="New chat"
              title="New chat"
            >
              <Plus class="w-4.5 h-4.5" />
            </button>
            <button
              type="button"
              onClick={onClose}
              class="md:hidden h-10 w-10 rounded-md bg-white/5 text-ink-100 grid place-items-center
                     hover:bg-white/10 active:scale-[0.98] transition"
              aria-label="Close chats"
              title="Close"
            >
              <X class="w-4.5 h-4.5" />
            </button>
          </div>

          <label class="mt-3 flex items-center gap-2 h-10 rounded-md bg-[#0b0d11] border border-white/10 px-3
                        focus-within:border-accent-blue/70 transition-colors">
            <Search class="w-4 h-4 text-ink-300 flex-none" />
            <input
              value={query}
              onInput={(e) => setQuery((e.currentTarget as HTMLInputElement).value)}
              placeholder="Search chats"
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
          <span>{filtered.length === chats.length ? `${chats.length} chats` : `${filtered.length} matches`}</span>
          {sorted[0] && (
            <span class="inline-flex items-center gap-1 min-w-0">
              <Clock class="w-3.5 h-3.5 flex-none" />
              <span class="truncate">{timeAgo(sorted[0].lastMessageAt)}</span>
            </span>
          )}
        </div>

        <div class="flex-1 overflow-y-auto touch-scroll px-2 pb-3 space-y-1">
          {filtered.length === 0 && (
            <div class="mx-2 rounded-lg border border-dashed border-white/12 bg-white/[0.03] text-center text-ink-300 text-sm py-8 px-4">
              <MessageSquare class="w-8 h-8 mx-auto mb-3 opacity-50" />
              <div class="text-ink-100 font-medium">{query ? "No matching chats" : "No chats yet"}</div>
              <button
                type="button"
                onClick={newChat}
                class="mt-4 inline-flex items-center gap-1.5 h-9 px-3 rounded-md
                       bg-white/8 hover:bg-white/12 text-ink-100 text-sm"
              >
                <Plus class="w-4 h-4" /> New chat
              </button>
            </div>
          )}

          {filtered.map((c) => {
            const active = c.id === activeChatId;
            return (
              <div
                key={c.id}
                class={`group flex items-stretch gap-1 rounded-lg border transition-colors
                        ${active
                          ? "border-accent-blue/35 bg-accent-blue/12"
                          : "border-transparent hover:border-white/10 hover:bg-white/[0.04]"}`}
              >
                <button
                  type="button"
                  onClick={() => onSelect(c.id)}
                  class="flex-1 min-w-0 text-left px-3 py-3"
                >
                  <div class="flex items-start gap-2">
                    <div
                      class={`mt-0.5 h-8 w-8 rounded-md grid place-items-center flex-none
                              ${active ? "bg-accent-blue/20 text-accent-blue" : "bg-white/6 text-ink-300"}`}
                    >
                      <MessageSquare class="w-4 h-4" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class={`text-[14px] leading-snug truncate font-medium ${active ? "text-ink-50" : "text-ink-100"}`}>
                        {c.title || "Untitled"}
                      </div>
                      <div class="mt-1 flex items-center gap-2 text-[12px] text-ink-300">
                        <span class={`px-1.5 py-0.5 rounded bg-white/6 ${active ? "text-accent-blue" : ""}`}>
                          {modelLabel(c.model)}
                        </span>
                        <span>{timeAgo(c.lastMessageAt)}</span>
                      </div>
                      {c.cwd && (
                        <div class="mt-1 flex items-center gap-1.5 text-[12px] text-ink-300 min-w-0 font-mono">
                          <Folder class="w-3.5 h-3.5 flex-none text-ink-400" />
                          <span class="truncate">{shortenPath(c.cwd)}</span>
                        </div>
                      )}
                    </div>
                  </div>
                </button>
                <button
                  type="button"
                  onClick={(e) => deleteChat(c, e)}
                  class={`w-10 grid place-items-center rounded-r-lg text-ink-300 hover:text-accent-red hover:bg-accent-red/10
                          opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity`}
                  aria-label={`Delete ${c.title || "chat"}`}
                  title="Delete chat"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
            );
          })}
        </div>

        {auth && !auth.noAuth && auth.authenticated && (
          <footer class="safe-bottom border-t border-white/10 px-3 py-3 flex items-center gap-2 text-sm bg-[#0d1015]">
            <div class="w-9 h-9 rounded-md bg-accent-green/15 text-accent-green
                        grid place-items-center font-semibold flex-none">
              {(auth.email[0] || "?").toUpperCase()}
            </div>
            <span class="flex-1 min-w-0 truncate text-ink-200" title={auth.email}>{auth.email}</span>
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
