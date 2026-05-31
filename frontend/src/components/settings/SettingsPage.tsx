import type { AppearanceTheme } from "../../models/settings";
import type { CodexDeviceLogin } from "../../models/auth";
import { ChevronLeft, Menu } from "../ui/icons";
import { AppearanceSettings } from "./AppearanceSettings";
import { CodexAuthSettings } from "./CodexAuthSettings";

export function SettingsPage({
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
  onBack,
  onHamburger,
  onAppearanceThemeChange,
  onStartCodexDeviceLogin,
}: {
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
  onBack: () => void;
  onHamburger: () => void;
  onAppearanceThemeChange: (theme: AppearanceTheme) => void;
  onStartCodexDeviceLogin: () => Promise<void>;
}) {
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

      <div class="flex-1 overflow-y-auto touch-scroll">
        <div class="max-w-2xl mx-auto px-4 py-5 space-y-4">
          <AppearanceSettings
            theme={appearanceTheme}
            loading={appearanceLoading}
            saving={appearanceSaving}
            error={appearanceError}
            onThemeChange={onAppearanceThemeChange}
          />

          <CodexAuthSettings
            authenticated={codexAuthenticated}
            usesApiKey={codexUsesApiKey}
            deviceLogin={codexDeviceLogin}
            loading={codexLoading}
            starting={codexStarting}
            error={codexError}
            onStartDeviceLogin={onStartCodexDeviceLogin}
          />

          <p class="text-[13px] leading-relaxed text-ink-300">
            Project credentials (GitHub PAT, Cloudflare token, Hetzner token, GCP service-account JSON)
            are now configured <strong class="text-ink-100">per project</strong> from each project's
            Containers page. That keeps account boundaries clean — project A can target one Cloudflare
            account, project B another, without conflating them.
          </p>
        </div>
      </div>
    </div>
  );
}
