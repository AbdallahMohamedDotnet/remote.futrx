import type { AppearanceTheme } from "../../models/settings";
import type { CodexDeviceLogin, KimiDeviceLogin } from "../../models/auth";
import type { UserDirectory } from "../../state/hooks/users/useUserDirectory";
import type { ServerInfo } from "../../models/serverInfo";
import type { ComponentType } from "preact";
import { Bot, ChevronLeft, Info, Menu, Monitor, Users } from "../primitives/icons";
import { AppearanceSettings } from "./AppearanceSettings";
import { ClaudeAuthSettings } from "./ClaudeAuthSettings";
import { CodexAuthSettings } from "./CodexAuthSettings";
import { KimiAuthSettings } from "./KimiAuthSettings";
import { ServerInfoSettings } from "./ServerInfoSettings";
import { UsersPanel } from "../account/UsersPanel";

export type SettingsTab = "appearance" | "agents" | "users" | "info";

const tabs: Array<{
  id: SettingsTab;
  label: string;
  description: string;
  Icon: ComponentType<{ class?: string }>;
}> = [
  {
    id: "appearance",
    label: "Appearance",
    description: "Choose how Remote looks on this device.",
    Icon: Monitor,
  },
  {
    id: "agents",
    label: "Agents",
    description: "Manage host authentication for coding agents.",
    Icon: Bot,
  },
  {
    id: "users",
    label: "Users",
    description: "Control who can access this server.",
    Icon: Users,
  },
  {
    id: "info",
    label: "Info",
    description: "View details about the main parent server.",
    Icon: Info,
  },
];

export function SettingsPage({
  activeTab,
  currentEmail,
  isAdmin,
  noAuth,
  serverInfo,
  serverInfoLoading,
  serverInfoRefreshing,
  serverInfoError,
  userDirectory,
  appearanceTheme,
  appearanceLoading,
  appearanceSaving,
  appearanceError,
  codexAuthenticated,
  codexUsesApiKey,
  codexDeviceLogin,
  codexLoading,
  codexStarting,
  codexError,
  kimiAuthenticated,
  kimiDeviceLogin,
  kimiLoading,
  kimiStarting,
  kimiError,
  onBack,
  onHamburger,
  onTabChange,
  onRefreshServerInfo,
  onAppearanceThemeChange,
  onStartCodexDeviceLogin,
  onStartKimiDeviceLogin,
}: {
  activeTab: SettingsTab;
  currentEmail: string;
  isAdmin: boolean;
  noAuth: boolean;
  serverInfo: ServerInfo | null;
  serverInfoLoading: boolean;
  serverInfoRefreshing: boolean;
  serverInfoError: string | null;
  userDirectory: UserDirectory;
  appearanceTheme: AppearanceTheme;
  appearanceLoading: boolean;
  appearanceSaving: boolean;
  appearanceError: string | null;
  codexAuthenticated: boolean;
  codexUsesApiKey: boolean;
  codexDeviceLogin?: CodexDeviceLogin;
  codexLoading: boolean;
  codexStarting: boolean;
  codexError: string | null;
  kimiAuthenticated: boolean;
  kimiDeviceLogin?: KimiDeviceLogin;
  kimiLoading: boolean;
  kimiStarting: boolean;
  kimiError: string | null;
  onBack: () => void;
  onHamburger: () => void;
  onTabChange: (tab: SettingsTab) => void;
  onRefreshServerInfo: () => Promise<void>;
  onAppearanceThemeChange: (theme: AppearanceTheme) => void;
  onStartCodexDeviceLogin: () => Promise<void>;
  onStartKimiDeviceLogin: () => Promise<void>;
}) {
  const activeTabDetails = tabs.find((tab) => tab.id === activeTab) ?? tabs[0];

  return (
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
      <header class="codex-header top-chrome flex-none z-20 bg-[#101318] border-b border-white/10 px-3 pb-2 flex items-center gap-2 min-h-[52px]">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 text-ink-100 rounded-md hover:bg-white/[0.08] grid place-items-center"
          aria-label="Toggle sidebar"
        >
          <Menu class="w-5 h-5" />
        </button>
        <button
          type="button"
          onClick={onBack}
          class="hidden md:inline-flex items-center gap-1.5 h-10 px-2 text-ink-200 hover:text-ink-50
                 hover:bg-white/[0.08] rounded-md text-sm"
        >
          <ChevronLeft class="w-4 h-4" /> Chats
        </button>
        <div class="flex-1 min-w-0">
          <div class="text-[11px] text-ink-300">Preferences</div>
          <div class="text-[15px] font-semibold text-ink-50 truncate">Settings</div>
        </div>
      </header>

      <div class="flex-1 min-h-0 flex flex-col md:flex-row overflow-hidden">
        <SettingsNavigation
          activeTab={activeTab}
          onTabChange={onTabChange}
          className="hidden md:flex w-56 flex-none border-r border-white/10 bg-[#0f1217] p-3"
        />
        <SettingsNavigation
          activeTab={activeTab}
          onTabChange={onTabChange}
          mobile
          className="md:hidden flex-none border-b border-white/10 bg-[#0f1217] px-3 py-2 overflow-x-auto no-scrollbar"
        />

        <main
          id="settings-active-panel"
          role="tabpanel"
          class="flex-1 min-w-0 overflow-y-auto touch-scroll"
        >
          <div class="w-full px-4 py-5 md:px-6 md:py-7">
            <header class="mb-5">
              <h1 class="text-xl font-semibold text-ink-50">{activeTabDetails.label}</h1>
              <p class="mt-1 text-[13px] leading-relaxed text-ink-300">
                {activeTabDetails.description}
              </p>
            </header>

            {activeTab === "appearance" && (
              <AppearanceSettings
                theme={appearanceTheme}
                loading={appearanceLoading}
                saving={appearanceSaving}
                error={appearanceError}
                onThemeChange={onAppearanceThemeChange}
              />
            )}

            {activeTab === "agents" && (
              isAdmin ? (
                <div class="rounded-lg border border-white/10 bg-[#101318] overflow-hidden">
                  <div class="px-4 py-3 border-b border-white/[0.06]">
                    <div class="text-[14.5px] font-semibold text-ink-50">Agent authentication</div>
                    <div class="text-[12.5px] text-ink-300 mt-0.5 leading-snug">
                      Sign in once on the parent host and share the credentials with project containers.
                    </div>
                  </div>
                  <div class="p-3 space-y-3">
                    <ClaudeAuthSettings />
                    <CodexAuthSettings
                      authenticated={codexAuthenticated}
                      usesApiKey={codexUsesApiKey}
                      deviceLogin={codexDeviceLogin}
                      loading={codexLoading}
                      starting={codexStarting}
                      error={codexError}
                      onStartDeviceLogin={onStartCodexDeviceLogin}
                    />
                    <KimiAuthSettings
                      authenticated={kimiAuthenticated}
                      deviceLogin={kimiDeviceLogin}
                      loading={kimiLoading}
                      starting={kimiStarting}
                      error={kimiError}
                      onStartDeviceLogin={onStartKimiDeviceLogin}
                    />
                  </div>
                </div>
              ) : (
                <SettingsNotice>
                  Agent authentication is managed by server administrators.
                </SettingsNotice>
              )
            )}

            {activeTab === "users" && (
              noAuth ? (
                <SettingsNotice>
                  User management is unavailable while authentication is disabled on this server.
                </SettingsNotice>
              ) : (
                <UsersPanel
                  currentEmail={currentEmail}
                  isAdmin={isAdmin}
                  users={userDirectory.users}
                  loading={userDirectory.loading}
                  error={userDirectory.error}
                  onAdd={userDirectory.add}
                  onRemove={userDirectory.remove}
                  onSetRole={userDirectory.setRole}
                />
              )
            )}

            {activeTab === "info" && (
              <ServerInfoSettings
                currentEmail={currentEmail}
                isAdmin={isAdmin}
                noAuth={noAuth}
                info={serverInfo}
                loading={serverInfoLoading}
                refreshing={serverInfoRefreshing}
                error={serverInfoError}
                onRefresh={onRefreshServerInfo}
              />
            )}
          </div>
        </main>
      </div>
    </div>
  );
}

function SettingsNavigation({
  activeTab,
  onTabChange,
  mobile = false,
  className,
}: {
  activeTab: SettingsTab;
  onTabChange: (tab: SettingsTab) => void;
  mobile?: boolean;
  className: string;
}) {
  return (
    <aside class={className} aria-label="Settings sections">
      <nav
        class={mobile ? "flex gap-1 min-w-max" : "w-full space-y-1"}
        role="tablist"
        aria-orientation={mobile ? "horizontal" : "vertical"}
      >
        {tabs.map(({ id, label, Icon }) => {
          const active = id === activeTab;
          return (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={active}
              aria-controls="settings-active-panel"
              tabIndex={active ? 0 : -1}
              onClick={() => onTabChange(id)}
              class={`${mobile ? "h-9 px-3" : "w-full h-10 px-3"} rounded-md inline-flex items-center gap-2.5 border text-[13px] font-medium transition-colors ${
                active
                  ? "border-white/10 bg-white/[0.08] text-ink-50"
                  : "border-transparent text-ink-300 hover:text-ink-100 hover:bg-white/[0.05]"
              }`}
            >
              <Icon class={`w-4 h-4 flex-none ${active ? "text-accent-blue" : "text-ink-400"}`} />
              <span>{label}</span>
            </button>
          );
        })}
      </nav>
    </aside>
  );
}

function SettingsNotice({ children }: { children: string }) {
  return (
    <section class="rounded-lg border border-white/10 bg-[#101318] p-4 text-[13px] leading-relaxed text-ink-300">
      {children}
    </section>
  );
}
