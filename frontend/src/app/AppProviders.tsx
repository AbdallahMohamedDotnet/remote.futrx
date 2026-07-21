import type { ComponentChildren } from "preact";
import { AuthProvider } from "../state/context/AuthContext";
import { UserSettingsProvider } from "../state/context/UserSettingsContext";

export function AppProviders({ children }: { children: ComponentChildren }) {
  return (
    <AuthProvider>
      <UserSettingsProvider>{children}</UserSettingsProvider>
    </AuthProvider>
  );
}
