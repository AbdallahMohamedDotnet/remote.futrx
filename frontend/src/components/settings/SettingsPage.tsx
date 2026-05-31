import type { AppearanceTheme, CredentialProvider, ProviderKey } from "../../models/settings";
import { AlertCircle, ChevronLeft, Menu } from "../ui/icons";
import { AppearanceSettings } from "./AppearanceSettings";
import { ProviderCard } from "./ProviderCard";

export function SettingsPage({
  providers,
  appearanceTheme,
  appearanceLoading,
  appearanceSaving,
  appearanceError,
  values,
  expandedHelp,
  revealed,
  savedAt,
  onBack,
  onHamburger,
  onAppearanceThemeChange,
  onValueChange,
  onToggleHelp,
  onToggleReveal,
  onSave,
}: {
  providers: CredentialProvider[];
  appearanceTheme: AppearanceTheme;
  appearanceLoading: boolean;
  appearanceSaving: boolean;
  appearanceError: string | null;
  values: Record<ProviderKey, string>;
  expandedHelp: Record<ProviderKey, boolean>;
  revealed: Record<ProviderKey, boolean>;
  savedAt: Record<ProviderKey, number | undefined>;
  onBack: () => void;
  onHamburger: () => void;
  onAppearanceThemeChange: (theme: AppearanceTheme) => void;
  onValueChange: (key: ProviderKey, value: string) => void;
  onToggleHelp: (key: ProviderKey) => void;
  onToggleReveal: (key: ProviderKey) => void;
  onSave: (key: ProviderKey) => void;
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

          <p class="text-[13px] leading-relaxed text-ink-300">
            These credentials are <strong class="text-ink-100">host-wide</strong>. Paste once
            and every project's container gets seeded so <code class="font-mono text-ink-100">gh</code>,{" "}
            <code class="font-mono text-ink-100">wrangler</code>,{" "}
            <code class="font-mono text-ink-100">hcloud</code>, and{" "}
            <code class="font-mono text-ink-100">gcloud</code> work inside the sandbox.
            Use long-lived tokens or service-account keys, not interactive logins.
          </p>

          <div class="flex items-start gap-2.5 rounded-lg border border-accent-yellow/30 bg-accent-yellow/[0.08]
                      text-ink-100 px-3 py-2.5 text-[13px] leading-relaxed">
            <AlertCircle class="w-4 h-4 mt-0.5 flex-none text-accent-yellow" />
            <div>
              <div class="font-medium text-accent-yellow">Backend storage not wired yet</div>
              <div class="text-ink-200 mt-0.5">
                Credential values are not persisted or pushed to containers. This section is the UI stub.
              </div>
            </div>
          </div>

          {providers.map((provider) => (
            <ProviderCard
              key={provider.key}
              provider={provider}
              value={values[provider.key]}
              helpOpen={!!expandedHelp[provider.key]}
              revealed={!!revealed[provider.key]}
              savedAt={savedAt[provider.key]}
              onChange={(value) => onValueChange(provider.key, value)}
              onToggleHelp={() => onToggleHelp(provider.key)}
              onToggleReveal={() => onToggleReveal(provider.key)}
              onSave={() => onSave(provider.key)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
