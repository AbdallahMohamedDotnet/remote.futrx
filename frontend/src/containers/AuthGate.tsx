import type { ComponentType } from "preact";
import { useEffect, useState } from "preact/hooks";
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
    return <ClaudeLoginContainer onDone={claudeAuth.refresh} />;
  }

  if (!WorkspaceRoute) return <LoadingScreen />;
  return <WorkspaceRoute enabled={gateOpen} />;
}
