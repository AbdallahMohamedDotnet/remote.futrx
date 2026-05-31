import { SettingsPage } from "../components/settings/SettingsPage";
import { useUserSettingsContext } from "../context/UserSettingsContext";
import { useSettingsCredentials } from "../hooks/settings/useSettingsCredentials";
import { CREDENTIAL_PROVIDERS } from "../state/settings/providers";

export function SettingsContainer({
  onBack,
  onHamburger,
}: {
  onBack: () => void;
  onHamburger: () => void;
}) {
  const credentials = useSettingsCredentials();
  const userSettings = useUserSettingsContext();

  return (
    <SettingsPage
      providers={CREDENTIAL_PROVIDERS}
      appearanceTheme={userSettings.settings.appearance.theme}
      appearanceLoading={userSettings.loading}
      appearanceSaving={userSettings.saving}
      appearanceError={userSettings.error}
      values={credentials.values}
      expandedHelp={credentials.expandedHelp}
      revealed={credentials.revealed}
      savedAt={credentials.savedAt}
      stored={credentials.stored}
      saving={credentials.saving}
      errors={credentials.errors}
      propagation={credentials.propagation}
      onBack={onBack}
      onHamburger={onHamburger}
      onAppearanceThemeChange={(theme) => void userSettings.setTheme(theme)}
      onValueChange={credentials.setValue}
      onToggleHelp={credentials.toggleHelp}
      onToggleReveal={credentials.toggleReveal}
      onSave={credentials.save}
      onClear={credentials.clear}
    />
  );
}
