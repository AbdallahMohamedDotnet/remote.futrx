import type { ComponentType } from "preact";
import { useEffect, useState } from "preact/hooks";
import { LoginScreen } from "../../ui/auth/LoginScreen";
import { AdminSetupWaiting } from "../../ui/auth/AdminSetupWaiting";
import { ProviderAuthWaiting } from "../../ui/auth/ProviderAuthWaiting";
import { ProviderLoginScreen } from "../../ui/auth/ProviderLoginScreen";
import { LoadingScreen } from "../../ui/primitives/LoadingScreen";
import { useAuthContext } from "../../state/context/AuthContext";

type WorkspaceRouteComponent = ComponentType<{ enabled: boolean }>;

export function AuthGate() {
  const {
    auth,
    appAuthOk,
    providerAuthChecked,
    providerAuthenticated,
    gateOpen,
  } = useAuthContext();
  const [WorkspaceRoute, setWorkspaceRoute] = useState<WorkspaceRouteComponent | null>(null);

  useEffect(() => {
    if (!gateOpen || WorkspaceRoute) return;
    let cancelled = false;
    import("../routes/WorkspaceRoute").then((module) => {
      if (!cancelled) setWorkspaceRoute(() => module.WorkspaceRoute);
    });
    return () => {
      cancelled = true;
    };
  }, [gateOpen, WorkspaceRoute]);

  if (auth.loading) return <LoadingScreen />;
  if (!auth.claimed) {
    return (
      <LoginScreen
        mode="claim"
        adminEmail=""
        localAdminConfigured={false}
        googleOAuthEnabled={false}
        onSuccess={auth.refresh}
      />
    );
  }
  if (!appAuthOk) {
    return (
      <LoginScreen
        mode="login"
        adminEmail={auth.adminEmail}
        localAdminConfigured={auth.localAdminConfigured}
        googleOAuthEnabled={auth.googleOAuthEnabled}
        onSuccess={auth.refresh}
      />
    );
  }
  if (!auth.localAdminConfigured) {
    if (auth.isAdmin && auth.email.toLowerCase() === auth.adminEmail.toLowerCase()) {
      return (
        <LoginScreen
          mode="legacy-setup"
          adminEmail={auth.email}
          localAdminConfigured={false}
          googleOAuthEnabled={auth.googleOAuthEnabled}
          onSuccess={auth.refresh}
        />
      );
    }
    return <AdminSetupWaiting adminEmail={auth.adminEmail} />;
  }
  if (!providerAuthChecked) return <LoadingScreen />;
  if (!providerAuthenticated) {
    // Provider credentials are host-wide and admin-managed. Members wait while
    // an admin connects any one of the supported providers.
    if (auth.isAdmin) {
      return <ProviderLoginScreen />;
    }
    return <ProviderAuthWaiting adminEmail={auth.adminEmail} />;
  }

  if (!WorkspaceRoute) return <LoadingScreen />;
  return <WorkspaceRoute enabled={gateOpen} />;
}
