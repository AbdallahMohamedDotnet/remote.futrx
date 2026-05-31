import { SettingsPage } from "../components/settings/SettingsPage";
import { useUserSettingsContext } from "../context/UserSettingsContext";
import { useCodexAuth } from "../hooks/auth/useCodexAuth";

export function SettingsContainer({
  onBack,
  onHamburger,
}: {
  onBack: () => void;
  onHamburger: () => void;
}) {
  const userSettings = useUserSettingsContext();
  const codexAuth = useCodexAuth(true);

  return (
    <SettingsPage
      appearanceTheme={userSettings.settings.appearance.theme}
      appearanceLoading={userSettings.loading}
      appearanceSaving={userSettings.saving}
      appearanceError={userSettings.error}
      codexAuthenticated={codexAuth.authenticated}
      codexUsesApiKey={codexAuth.usesApiKey}
      codexDeviceLogin={codexAuth.deviceLogin}
      codexLoading={codexAuth.loading}
      codexStarting={codexAuth.starting}
      codexError={codexAuth.error}
      onBack={onBack}
      onHamburger={onHamburger}
      onAppearanceThemeChange={(theme) => void userSettings.setTheme(theme)}
      onStartCodexDeviceLogin={codexAuth.startDeviceLogin}
    />
  );
}
