import { useEffect, useState } from "preact/hooks";
import { fetchAuthSession } from "../../../api/authApi";
import type { AuthSession } from "../../../models/auth";

export interface AuthState extends AuthSession {
  loading: boolean;
  refresh: () => Promise<void>;
}

const initial: AuthState = {
  loading: true,
  noAuth: false,
  authenticated: false,
  claimed: false,
  adminEmail: "",
  email: "",
  isAdmin: false,
  isRegistered: false,
  refresh: async () => {},
};

export function useAuth(): AuthState {
  const [state, setState] = useState<AuthState>(initial);

  async function load() {
    const session = await fetchAuthSession();
    setState({ ...session, loading: false, refresh: load });
  }

  useEffect(() => { load(); }, []);

  return { ...state, refresh: load };
}
