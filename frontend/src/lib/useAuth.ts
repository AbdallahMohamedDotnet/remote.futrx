import { useEffect, useState } from "preact/hooks";

export interface AuthState {
  loading: boolean;
  // True when the server has no auth backend (oauth.json absent). The whole
  // app is open; no login screen needed. Matches legacy deployments.
  noAuth: boolean;
  authenticated: boolean;
  // True if any user has claimed this server yet.
  claimed: boolean;
  // The admin's email — shown on the "this server is claimed by …" message
  // when a non-admin tries to log in. Empty if unclaimed.
  adminEmail: string;
  // Current user (only when authenticated).
  email: string;
  isAdmin: boolean;
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
  refresh: async () => {},
};

export function useAuth(): AuthState {
  const [state, setState] = useState<AuthState>(initial);

  async function load() {
    try {
      const r = await fetch("/auth/me", { credentials: "same-origin" });
      if (r.status === 404) {
        // Server has no /auth/me handler => auth backend disabled.
        setState({ ...initial, loading: false, noAuth: true, authenticated: true, isAdmin: true });
        return;
      }
      if (!r.ok) {
        setState({ ...initial, loading: false });
        return;
      }
      const d = await r.json();
      setState({
        loading: false,
        noAuth: false,
        authenticated: !!d.authenticated,
        claimed: !!d.claimed,
        adminEmail: d.adminEmail ?? "",
        email: d.email ?? "",
        isAdmin: !!d.isAdmin,
        refresh: load,
      });
    } catch {
      setState({ ...initial, loading: false });
    }
  }

  useEffect(() => { load(); }, []);

  return { ...state, refresh: load };
}
