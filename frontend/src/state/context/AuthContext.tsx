import type { ComponentChildren } from "preact";
import { createContext } from "preact";
import { useContext } from "preact/hooks";
import { useAuth, type AuthState } from "../../hooks/auth/useAuth";
import { useClaudeAuth, type ClaudeAuthState } from "../../hooks/auth/useClaudeAuth";

interface AuthContextValue {
  auth: AuthState;
  claudeAuth: ClaudeAuthState;
  googleOk: boolean;
  gateOpen: boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ComponentChildren }) {
  const auth = useAuth();
  // Any registered user can use the workspace. Admins are still registered
  // by definition; noAuth (oauth.json absent) opens the door for solo dev.
  const googleOk = auth.authenticated && (auth.isRegistered || auth.isAdmin || auth.noAuth);
  const claudeAuth = useClaudeAuth(googleOk);
  const gateOpen = googleOk && claudeAuth.authenticated;

  return (
    <AuthContext.Provider value={{ auth, claudeAuth, googleOk, gateOpen }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuthContext(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuthContext must be used inside AuthProvider");
  return value;
}
