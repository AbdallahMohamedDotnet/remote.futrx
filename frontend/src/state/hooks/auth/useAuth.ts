import { useCallback, useEffect, useState } from "preact/hooks";
import { fetchAuthSession } from "../../../api/authApi";
import type { AuthSession } from "../../../models/auth";

export interface AuthState extends AuthSession {
  loading: boolean;
  refresh: () => Promise<void>;
}

const initial: Omit<AuthState, "refresh"> = {
  loading: true,
  authenticated: false,
  claimed: false,
  localAdminConfigured: false,
  googleOAuthEnabled: false,
  googleClientId: "",
  adminEmail: "",
  email: "",
  isAdmin: false,
  isRegistered: false,
};

export function useAuth(): AuthState {
  const [state, setState] = useState<Omit<AuthState, "refresh">>(initial);

  const refresh = useCallback(async () => {
    const session = await fetchAuthSession();
    setState({ ...session, loading: false });
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  return { ...state, refresh };
}
