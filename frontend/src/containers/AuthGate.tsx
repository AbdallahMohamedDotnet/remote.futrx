import type { ComponentType } from "preact";
import { useEffect, useState } from "preact/hooks";
import { ClaudeAuthWaiting } from "../components/auth/ClaudeAuthWaiting";
import { LoginScreen } from "../components/auth/LoginScreen";
import { LoadingScreen } from "../components/ui/LoadingScreen";
import { useAuthContext } from "../context/AuthContext";
import { ClaudeLoginContainer } from "./ClaudeLoginContainer";

type WorkspaceRouteComponent = ComponentType<{ enabled: boolean }>;

export function AuthGate() {
  const { auth, claudeAuth, googleOk, gateOpen } = useAuthContext();
  const [WorkspaceRoute, setWorkspaceRoute] = useState<WorkspaceRouteComponent | null>(null);

  useEffect(() => {
    if (!gateOpen || WorkspaceRoute) return;
    let cancelled = false;
    import("../app/routes/WorkspaceRoute").then((module) => {
      if (!cancelled) setWorkspaceRoute(() => module.WorkspaceRoute);
    });
    return () => {
      cancelled = true;
    };
  }, [gateOpen, WorkspaceRoute]);

  if (auth.loading) return <LoadingScreen />;
  if (!googleOk) {
    return <LoginScreen claimed={auth.claimed} adminEmail={auth.adminEmail} />;
  }
  if (!claudeAuth.checked || claudeAuth.loading) return <LoadingScreen />;
  if (!claudeAuth.authenticated) {
    // Claude login is admin-only (host-wide credential). Non-admins wait here
    // until an admin signs in; the live status WS opens the gate for them.
    if (auth.isAdmin || auth.noAuth) {
      return <ClaudeLoginContainer onDone={claudeAuth.refresh} />;
    }
    return <ClaudeAuthWaiting adminEmail={auth.adminEmail} />;
  }

  if (!WorkspaceRoute) return <LoadingScreen />;
  return <WorkspaceRoute enabled={gateOpen} />;
}
