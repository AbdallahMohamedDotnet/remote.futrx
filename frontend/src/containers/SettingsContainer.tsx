import { SettingsPage } from "../components/settings/SettingsPage";
import { useUserSettingsContext } from "../context/UserSettingsContext";

export function SettingsContainer({
  onBack,
  onHamburger,
}: {
  onBack: () => void;
  onHamburger: () => void;
}) {
  const userSettings = useUserSettingsContext();

  return (
    <SettingsPage
      appearanceTheme={userSettings.settings.appearance.theme}
      appearanceLoading={userSettings.loading}
      appearanceSaving={userSettings.saving}
      appearanceError={userSettings.error}
      onBack={onBack}
      onHamburger={onHamburger}
      onAppearanceThemeChange={(theme) => void userSettings.setTheme(theme)}
    />
  );
}
