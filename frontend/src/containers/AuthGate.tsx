import { LoginScreen } from "../components/auth/LoginScreen";
import { LoadingScreen } from "../components/ui/LoadingScreen";
import { useAuthContext } from "../context/AuthContext";
import { WorkspaceRoute } from "../app/routes/WorkspaceRoute";
import { ClaudeLoginContainer } from "./ClaudeLoginContainer";

export function AuthGate() {
  const { auth, claudeAuth, googleOk, gateOpen } = useAuthContext();

  if (auth.loading) return <LoadingScreen />;
  if (!googleOk) {
    return <LoginScreen claimed={auth.claimed} adminEmail={auth.adminEmail} />;
  }
  if (!claudeAuth.checked || claudeAuth.loading) return <LoadingScreen />;
  if (!claudeAuth.authenticated) {
    return <ClaudeLoginContainer onDone={claudeAuth.refresh} />;
  }

  return <WorkspaceRoute enabled={gateOpen} />;
}
