import type { ComponentChildren } from "preact";
import { AuthProvider } from "../context/AuthContext";
import { UserSettingsProvider } from "../context/UserSettingsContext";

export function AppProviders({ children }: { children: ComponentChildren }) {
  return (
    <AuthProvider>
      <UserSettingsProvider>{children}</UserSettingsProvider>
    </AuthProvider>
  );
}
