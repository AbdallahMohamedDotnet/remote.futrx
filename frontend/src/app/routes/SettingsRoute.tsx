import { SettingsContainer } from "../../state/containers/SettingsContainer";

export function SettingsRoute({
  onBack,
  onHamburger,
}: {
  onBack: () => void;
  onHamburger: () => void;
}) {
  return <SettingsContainer onBack={onBack} onHamburger={onHamburger} />;
}
