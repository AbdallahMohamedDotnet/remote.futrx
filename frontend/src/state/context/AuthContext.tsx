import type { ComponentChildren } from "preact";
import { createContext } from "preact";
import { useContext, useEffect, useRef } from "preact/hooks";
import { agentCapabilityCatalogStore } from "../agents/agentCapabilityCatalog";
import { useAuth, type AuthState } from "../hooks/auth/useAuth";
import { useClaudeAuth, type ClaudeAuthState } from "../hooks/auth/useClaudeAuth";
import { useCodexAuth, type CodexAuthState } from "../hooks/auth/useCodexAuth";
import { useKimiAuth, type KimiAuthState } from "../hooks/auth/useKimiAuth";

interface AuthContextValue {
  auth: AuthState;
  claudeAuth: ClaudeAuthState;
  codexAuth: CodexAuthState;
  kimiAuth: KimiAuthState;
  appAuthOk: boolean;
  providerAuthChecked: boolean;
  providerAuthenticated: boolean;
  gateOpen: boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

interface ProviderAuthMarker {
  userId: string;
  claudeAuthenticated: boolean;
  claudeCompleted: string;
  codexAuthenticated: boolean;
  codexCompleted: string;
  kimiAuthenticated: boolean;
  kimiCompleted: string;
}

export function AuthProvider({ children }: { children: ComponentChildren }) {
  const auth = useAuth();
  // A valid local-admin or invited-user session may proceed to provider setup.
  const appAuthOk = auth.authenticated && (auth.isRegistered || auth.isAdmin);
  const providerAuthEnabled = appAuthOk && auth.localAdminConfigured;
  const claudeAuth = useClaudeAuth(providerAuthEnabled);
  const codexAuth = useCodexAuth(providerAuthEnabled);
  const kimiAuth = useKimiAuth(providerAuthEnabled);
  const providerAuthChecked = claudeAuth.checked && codexAuth.checked && kimiAuth.checked;
  const providerAuthenticated =
    claudeAuth.authenticated || codexAuth.authenticated || kimiAuth.authenticated;
  const gateOpen = providerAuthEnabled && providerAuthChecked && providerAuthenticated;
  const previousProviderAuth = useRef<ProviderAuthMarker | null>(null);

  useEffect(() => {
    const userId = auth.email || auth.adminEmail;
    if (!providerAuthChecked || !userId) return;

    const current: ProviderAuthMarker = {
      userId: userId.trim().toLowerCase(),
      claudeAuthenticated: claudeAuth.authenticated,
      claudeCompleted: completionToken(claudeAuth.login?.completed, claudeAuth.login?.startedAt),
      codexAuthenticated: codexAuth.authenticated,
      codexCompleted: completionToken(
        codexAuth.deviceLogin?.completed,
        codexAuth.deviceLogin?.startedAt,
      ),
      kimiAuthenticated: kimiAuth.authenticated,
      kimiCompleted: completionToken(
        kimiAuth.deviceLogin?.completed,
        kimiAuth.deviceLogin?.startedAt,
      ),
    };
    const previous = previousProviderAuth.current;
    previousProviderAuth.current = current;
    if (!previous || previous.userId !== current.userId) return;

    const authenticationChanged =
      previous.claudeAuthenticated !== current.claudeAuthenticated
      || previous.codexAuthenticated !== current.codexAuthenticated
      || previous.kimiAuthenticated !== current.kimiAuthenticated;
    const loginCompleted =
      (!!current.claudeCompleted && current.claudeCompleted !== previous.claudeCompleted)
      || (!!current.codexCompleted && current.codexCompleted !== previous.codexCompleted)
      || (!!current.kimiCompleted && current.kimiCompleted !== previous.kimiCompleted);
    if (authenticationChanged || loginCompleted) {
      agentCapabilityCatalogStore.invalidateUser(current.userId);
    }
  }, [
    auth.email,
    auth.adminEmail,
    providerAuthChecked,
    claudeAuth.authenticated,
    claudeAuth.login?.completed,
    claudeAuth.login?.startedAt,
    codexAuth.authenticated,
    codexAuth.deviceLogin?.completed,
    codexAuth.deviceLogin?.startedAt,
    kimiAuth.authenticated,
    kimiAuth.deviceLogin?.completed,
    kimiAuth.deviceLogin?.startedAt,
  ]);

  return (
    <AuthContext.Provider
      value={{
        auth,
        claudeAuth,
        codexAuth,
        kimiAuth,
        appAuthOk,
        providerAuthChecked,
        providerAuthenticated,
        gateOpen,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

function completionToken(completed?: boolean, startedAt?: number): string {
  return completed ? String(startedAt || "completed") : "";
}

export function useAuthContext(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuthContext must be used inside AuthProvider");
  return value;
}
