import { useEffect, useState } from "preact/hooks";
import { ChatSidebar } from "./components/ChatSidebar";
import { ChatView } from "./components/Chat/ChatView";
import { LoginScreen } from "./components/LoginScreen";
import { chatsApi } from "./lib/api";
import { useAuth } from "./lib/useAuth";
import { usePoll } from "./lib/usePoll";
import type { ChatMeta } from "./types";
import { Loader, Menu, MessageSquare, Plus } from "./components/icons";

export function App() {
  const auth = useAuth();

  // Don't fetch chats until we know the user is allowed in.
  const canFetch = auth.authenticated && (auth.isAdmin || auth.noAuth);
  const { value: chats, refresh } = usePoll<ChatMeta[]>(
    () => (canFetch ? chatsApi.list() : Promise.resolve([])),
    8000,
    []
  );

  if (auth.loading) {
    return (
      <div class="h-full grid place-items-center text-ink-300">
        <Loader class="w-6 h-6 animate-spin" />
      </div>
    );
  }
  if (!auth.authenticated || (!auth.isAdmin && !auth.noAuth)) {
    return <LoginScreen auth={auth} />;
  }

  const [activeId, setActiveId] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);

  // Auto-select first chat on initial load, only if user hasn't picked.
  useEffect(() => {
    if (activeId === null && chats.length > 0) {
      setActiveId(chats[0].id);
    }
  }, [chats, activeId]);

  // Clear selection if active chat was deleted elsewhere.
  useEffect(() => {
    if (activeId && !chats.some((c) => c.id === activeId)) {
      setActiveId(null);
    }
  }, [chats, activeId]);

  const activeChat = activeId ? chats.find((c) => c.id === activeId) ?? null : null;

  async function newChat() {
    try {
      const c = await chatsApi.create({});
      refresh();
      setActiveId(c.id);
      setSidebarOpen(false);
    } catch (e) {
      alert("create failed: " + (e as Error).message);
    }
  }

  return (
    <div class="h-full flex bg-ink-800 text-ink-100">
      <ChatSidebar
        chats={chats}
        activeChatId={activeId}
        onSelect={(id) => { setActiveId(id); setSidebarOpen(false); }}
        onRefresh={refresh}
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        auth={auth}
      />
      <main class="flex-1 flex flex-col min-w-0">
        {activeChat ? (
          <ChatView
            key={activeChat.id}
            chat={activeChat}
            onHamburger={() => setSidebarOpen((o) => !o)}
            onMetaUpdate={refresh}
          />
        ) : (
          <EmptyState onNew={newChat} onHamburger={() => setSidebarOpen((o) => !o)} />
        )}
      </main>
    </div>
  );
}

function EmptyState({ onNew, onHamburger }: { onNew: () => void; onHamburger: () => void }) {
  return (
    <div class="flex-1 flex flex-col">
      <header class="bg-ink-700 border-b border-ink-500 px-3 py-2 flex items-center gap-2 min-h-[42px]">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden text-ink-100 p-1 rounded hover:bg-ink-600"
          aria-label="Toggle sidebar"
        >
          <Menu class="w-5 h-5" />
        </button>
        <span class="text-sm text-ink-200">No chat selected</span>
      </header>
      <div class="flex-1 grid place-items-center text-center p-6">
        <div class="space-y-4 max-w-sm">
          <MessageSquare class="w-12 h-12 mx-auto opacity-30" />
          <div class="text-ink-200">
            <div class="font-medium text-base text-ink-100">No chat selected</div>
            <div class="text-xs mt-2 leading-relaxed text-ink-300">
              Start a new conversation with Claude. Your messages, code edits, tool calls,
              and file uploads are all rendered inline.
            </div>
          </div>
          <button
            type="button"
            onClick={onNew}
            class="inline-flex items-center gap-1.5 bg-accent-blue hover:bg-accent-blue/85
                   text-white text-sm font-medium px-4 py-2 rounded"
          >
            <Plus class="w-4 h-4" /> New chat
          </button>
        </div>
      </div>
    </div>
  );
}
