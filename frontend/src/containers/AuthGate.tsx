import type { ComponentType } from "preact";
import { useEffect, useState } from "preact/hooks";
import { LoginScreen } from "../components/auth/LoginScreen";
import { LoadingScreen } from "../components/ui/LoadingScreen";
import { useAuthContext } from "../context/AuthContext";
import { ClaudeLoginContainer } from "./ClaudeLoginContainer";

type WorkspaceRouteComponent = ComponentType<{ enabled: boolean }>;
type TerminalRouteComponent = ComponentType<{ chatId: string }>;

function terminalChatId(): string | null {
  try {
    return new URLSearchParams(window.location.search).get("terminal");
  } catch {
    return null;
  }
}

export function AuthGate() {
  const { auth, claudeAuth, googleOk, gateOpen } = useAuthContext();
  const terminalId = terminalChatId();
  const [WorkspaceRoute, setWorkspaceRoute] = useState<WorkspaceRouteComponent | null>(null);
  const [TerminalRoute, setTerminalRoute] = useState<TerminalRouteComponent | null>(null);

  useEffect(() => {
    if (!gateOpen || terminalId || WorkspaceRoute) return;
    let cancelled = false;
    import("../app/routes/WorkspaceRoute").then((module) => {
      if (!cancelled) setWorkspaceRoute(() => module.WorkspaceRoute);
    });
    return () => {
      cancelled = true;
    };
  }, [gateOpen, terminalId, WorkspaceRoute]);

  useEffect(() => {
    if (!gateOpen || !terminalId || TerminalRoute) return;
    let cancelled = false;
    import("../app/routes/TerminalRoute").then((module) => {
      if (!cancelled) setTerminalRoute(() => module.TerminalRoute);
    });
    return () => {
      cancelled = true;
    };
  }, [gateOpen, terminalId, TerminalRoute]);

  if (auth.loading) return <LoadingScreen />;
  if (!googleOk) {
    return <LoginScreen claimed={auth.claimed} adminEmail={auth.adminEmail} />;
  }
  if (!claudeAuth.checked || claudeAuth.loading) return <LoadingScreen />;
  if (!claudeAuth.authenticated) {
    return <ClaudeLoginContainer onDone={claudeAuth.refresh} />;
  }

  if (terminalId) {
    if (!TerminalRoute) return <LoadingScreen />;
    return <TerminalRoute chatId={terminalId} />;
  }

  if (!WorkspaceRoute) return <LoadingScreen />;
  return <WorkspaceRoute enabled={gateOpen} />;
}
