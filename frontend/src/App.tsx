import { useEffect, useState } from "preact/hooks";
import { ChatSidebar } from "./components/ChatSidebar";
import { ChatView } from "./components/Chat/ChatView";
import { ClaudeLoginScreen } from "./components/ClaudeLoginScreen";
import { LoginScreen } from "./components/LoginScreen";
import { SettingsPage } from "./components/SettingsPage";
import { chatsApi, projectsApi } from "./lib/api";
import { useAuth } from "./lib/useAuth";
import { useClaudeAuth } from "./lib/useClaudeAuth";
import { usePoll } from "./lib/usePoll";
import type { ChatMeta, ProjectMeta } from "./types";
import { Folder, Loader, Menu, MessageSquare, Plus } from "./components/icons";

export function App() {
  const auth = useAuth();
  // Only ask the server about claude after Google auth is confirmed
  // (otherwise we'd hit the admin-only middleware and 401 → reload loop).
  const googleOk = auth.authenticated && (auth.isAdmin || auth.noAuth);
  const claudeAuth = useClaudeAuth(googleOk);

  // Gate everything on Google admin auth being good AND claude CLI being
  // authenticated on the server. Only then do we even fetch chats/projects.
  const gateOpen = googleOk && claudeAuth.authenticated;

  const { value: chats, refresh: refreshChats } = usePoll<ChatMeta[]>(
    () => (gateOpen ? chatsApi.list() : Promise.resolve([])),
    8000,
    []
  );
  const { value: projects, refresh: refreshProjects } = usePoll<ProjectMeta[]>(
    () => (gateOpen ? projectsApi.list() : Promise.resolve([])),
    8000,
    []
  );

  const [activeId, setActiveId] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [view, setView] = useState<"chat" | "settings">("chat");

  useEffect(() => {
    if (gateOpen) {
      refreshChats();
      refreshProjects();
    }
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

  async function newProject() {
    const name = prompt("Project name?", "");
    if (!name || !name.trim()) return;
    try {
      await projectsApi.create(name.trim());
      refreshProjects();
    } catch (e) {
      alert("create project failed: " + (e as Error).message);
    }
  }

  async function newChatInProject(projectId?: string) {
    try {
      const c = await chatsApi.create(projectId ? { projectId } : {});
      refreshChats();
      setActiveId(c.id);
      setSidebarOpen(false);
    } catch (e) {
      alert("create chat failed: " + (e as Error).message);
    }
  }

  return (
    <div class="codex-app app-shell relative flex bg-[#090b0f] text-ink-100 overflow-hidden">
      <ChatSidebar
        chats={chats}
        projects={projects}
        activeChatId={activeId}
        onSelect={(id) => { setActiveId(id); setSidebarOpen(false); setView("chat"); }}
        onRefreshChats={refreshChats}
        onRefreshProjects={refreshProjects}
        onNewProject={newProject}
        onNewChatInProject={newChatInProject}
        onOpenSettings={() => { setView("settings"); setSidebarOpen(false); }}
        open={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        auth={auth}
      />
      <main class="codex-main relative flex-1 flex flex-col min-w-0 h-full overflow-hidden bg-[#0b0d11]">
        {view === "settings" ? (
          <SettingsPage
            onBack={() => setView("chat")}
            onHamburger={() => setSidebarOpen(true)}
          />
        ) : activeChat ? (
          <ChatView
            key={activeChat.id}
            chat={activeChat}
            onHamburger={() => setSidebarOpen(true)}
            onMetaUpdate={refreshChats}
          />
        ) : (
          <EmptyState
            hasProjects={projects.length > 0}
            onNewProject={newProject}
            onNewChat={() => newChatInProject(undefined)}
            onHamburger={() => setSidebarOpen(true)}
          />
        )}
      </main>
    </div>
  );
}

function EmptyState({
  hasProjects,
  onNewProject,
  onNewChat,
  onHamburger,
}: {
  hasProjects: boolean;
  onNewProject: () => void;
  onNewChat: () => void;
  onHamburger: () => void;
}) {
  return (
    <div class="flex-1 flex flex-col min-h-0">
      <header class="codex-header top-chrome flex-none sticky top-0 z-20 bg-[#101318] border-b border-white/10 px-3 pb-2 flex items-center gap-2 min-h-[52px]">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 text-ink-100 rounded-md hover:bg-white/[0.08] grid place-items-center"
          aria-label="Toggle sidebar"
        >
          <Menu class="w-5 h-5" />
        </button>
        <span class="text-sm text-ink-200">No chat selected</span>
      </header>
      <div class="flex-1 grid place-items-center text-center p-5">
        <div class="space-y-5 max-w-sm">
          <div class="mx-auto w-16 h-16 rounded-lg bg-white/[0.06] border border-white/10 grid place-items-center">
            {hasProjects
              ? <MessageSquare class="w-8 h-8 opacity-70" />
              : <Folder class="w-8 h-8 opacity-70" />}
          </div>
          <div class="text-ink-200">
            <div class="font-semibold text-lg text-ink-50">
              {hasProjects ? "Choose a chat or start fresh" : "Create your first project"}
            </div>
            <div class="text-xs mt-2 leading-relaxed text-ink-300">
              {hasProjects
                ? "Pick a project on the left, then create a chat inside it. Each project is its own sandboxed container."
                : "Projects are sandboxed dev environments — claude installs and runs inside each project's container, isolated from the rest of the host."}
            </div>
          </div>
          <div class="flex gap-2 justify-center">
            <button
              type="button"
              onClick={onNewProject}
              class="inline-flex items-center gap-2 bg-accent-blue hover:bg-accent-blue/90 active:scale-[0.99]
                     text-white text-sm font-medium px-4 h-11 rounded-md transition"
            >
              <Folder class="w-4 h-4" /> New project
            </button>
            {hasProjects && (
              <button
                type="button"
                onClick={onNewChat}
                class="inline-flex items-center gap-2 bg-white/[0.08] hover:bg-white/[0.12]
                       text-ink-100 text-sm font-medium px-4 h-11 rounded-md transition"
              >
                <Plus class="w-4 h-4" /> Loose chat
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
