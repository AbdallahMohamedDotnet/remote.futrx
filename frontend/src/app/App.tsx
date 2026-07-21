import { AppProviders } from "./AppProviders";
import { AuthGate } from "../state/containers/AuthGate";

export function App() {
  return (
    <AppProviders>
      <AuthGate />
    </AppProviders>
  );
}
