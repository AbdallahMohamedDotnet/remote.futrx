import { AppProviders } from "./AppProviders";
import { AuthGate } from "../containers/AuthGate";

export function App() {
  return (
    <AppProviders>
      <AuthGate />
    </AppProviders>
  );
}
