import { useEffect, useState } from "preact/hooks";
import { ChatSidebar } from "./components/ChatSidebar";
import { ChatView } from "./components/Chat/ChatView";
import { ClaudeLoginScreen } from "./components/ClaudeLoginScreen";
import { LoginScreen } from "./components/LoginScreen";
import { chatsApi } from "./lib/api";
import { useAuth } from "./lib/useAuth";
import { useClaudeAuth } from "./lib/useClaudeAuth";
import { usePoll } from "./lib/usePoll";
import type { ChatMeta } from "./types";
import { Loader, Menu, MessageSquare, Plus } from "./components/icons";

export function App() {
  const auth = useAuth();
  // Only ask the server about claude after Google auth is confirmed
  // (otherwise we'd hit the admin-only middleware and 401 → reload loop).
  const googleOk = auth.authenticated && (auth.isAdmin || auth.noAuth);
  const claudeAuth = useClaudeAuth(googleOk);

  // Gate everything on Google admin auth being good AND claude CLI being
  // authenticated on the server. Only then do we even fetch chats.
  const gateOpen = googleOk && claudeAuth.authenticated;

  const { value: chats, refresh } = usePoll<ChatMeta[]>(
    () => (gateOpen ? chatsApi.list() : Promise.resolve([])),
    8000,
    []
  );

  const [activeId, setActiveId] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    if (gateOpen) refresh();
    // `refresh` always calls the latest polling function through usePoll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gateOpen]);

  // Auto-select first chat on initial load.
  useEffect(() => {
    if (gateOpen && activeId === null && chats.length > 0) {
      setActiveId(chats[0].id);
    }
  }, [chats, activeId, gateOpen]);

  // Clear selection if active chat was deleted elsewhere.
  useEffect(() => {
    if (activeId && !chats.some((c) => c.id === activeId)) {
      setActiveId(null);
    }
  }, [chats, activeId]);

  // --- gating UI --------------------------------------------------------
  const spinner = (
    <div class="app-shell grid place-items-center bg-[#090b0f] text-ink-300">
      <div class="grid place-items-center gap-3">
        <Loader class="w-6 h-6 animate-spin" />
        <span class="text-sm">Loading</span>
      </div>
    </div>
  );
  if (auth.loading) return spinner;
  if (!googleOk) return <LoginScreen auth={auth} />;
  // Google is good; wait for the first claude-auth check before deciding
  // whether to render the chat UI or the ClaudeLoginScreen.
  if (!claudeAuth.checked || claudeAuth.loading) return spinner;
  if (!claudeAuth.authenticated) {
    return <ClaudeLoginScreen onDone={claudeAuth.refresh} />;
  }

  // --- main app ---------------------------------------------------------
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
    <div class="app-shell flex bg-[#090b0f] text-ink-100 overflow-hidden">
      <ChatSidebar
        chats={chats}
        activeChatId={activeId}
        onSelect={(id) => { setActiveId(id); setSidebarOpen(false); }}
        onRefresh={refresh}
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        auth={auth}
      />
      <main class="relative flex-1 flex flex-col min-w-0 bg-[#0b0d11]">
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
    <div class="flex-1 flex flex-col min-h-0">
      <header class="safe-top bg-[#101318]/95 border-b border-white/10 px-3 py-2 flex items-center gap-2 min-h-[52px]">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 text-ink-100 rounded-md hover:bg-white/8 grid place-items-center"
          aria-label="Toggle sidebar"
        >
          <Menu class="w-5 h-5" />
        </button>
        <span class="text-sm text-ink-200">No chat selected</span>
      </header>
      <div class="flex-1 grid place-items-center text-center p-5">
        <div class="space-y-5 max-w-sm">
          <div class="mx-auto w-16 h-16 rounded-2xl bg-white/6 border border-white/10 grid place-items-center">
            <MessageSquare class="w-8 h-8 opacity-70" />
          </div>
          <div class="text-ink-200">
            <div class="font-semibold text-lg text-ink-50">Choose a chat or start fresh</div>
            <div class="text-xs mt-2 leading-relaxed text-ink-300">
              Messages, tool calls, file uploads, and code edits stay in one fast conversation view.
            </div>
          </div>
          <button
            type="button"
            onClick={onNew}
            class="inline-flex items-center gap-2 bg-accent-blue hover:bg-accent-blue/90 active:scale-[0.99]
                   text-white text-sm font-medium px-4 h-11 rounded-md transition"
          >
            <Plus class="w-4 h-4" /> New chat
          </button>
        </div>
      </div>
    </div>
  );
}
