import { SettingsContainer } from "../containers/SettingsContainer";

export function SettingsRoute({
  onBack,
  onHamburger,
}: {
  onBack: () => void;
  onHamburger: () => void;
}) {
  return <SettingsContainer onBack={onBack} onHamburger={onHamburger} />;
}
