import { SettingsPage } from "../../ui/settings/SettingsPage";
import { useAuthContext } from "../../state/context/AuthContext";
import { useUserSettingsContext } from "../../state/context/UserSettingsContext";
import { useCodexAuth } from "../../state/hooks/auth/useCodexAuth";
import { useKimiAuth } from "../../state/hooks/auth/useKimiAuth";

export function SettingsContainer({
  onBack,
  onHamburger,
}: {
  onBack: () => void;
  onHamburger: () => void;
}) {
  const { auth } = useAuthContext();
  const userSettings = useUserSettingsContext();
  const codexAuth = useCodexAuth(true);
  const kimiAuth = useKimiAuth(true);

  return (
    <SettingsPage
      currentEmail={auth.email}
      isAdmin={auth.isAdmin}
      noAuth={auth.noAuth}
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
      kimiAuthenticated={kimiAuth.authenticated}
      kimiDeviceLogin={kimiAuth.deviceLogin}
      kimiLoading={kimiAuth.loading}
      kimiStarting={kimiAuth.starting}
      kimiError={kimiAuth.error}
      onStartKimiDeviceLogin={kimiAuth.startDeviceLogin}
    />
  );
}
