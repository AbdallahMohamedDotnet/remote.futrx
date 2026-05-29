import type { ComponentChildren } from "preact";
import { AuthProvider } from "../context/AuthContext";

export function AppProviders({ children }: { children: ComponentChildren }) {
  return <AuthProvider>{children}</AuthProvider>;
}
