import { useEffect, useState } from "preact/hooks";
import { claudeAuthApi } from "./api";

export interface ClaudeAuthState {
  loading: boolean;
  // true once we've actually called the server at least once. Useful so the
  // gating UI can show a spinner BETWEEN the moment we know we should query
  // (googleOk became true) and the moment the first response lands.
  checked: boolean;
  authenticated: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

// Polls /api/claude/auth-status. Used by App.tsx to gate the chat UI on the
// claude CLI being authenticated against Anthropic on the server.
//
// `enabled` MUST be false until Google auth is confirmed — /api/claude/* is
// admin-only at the server middleware. Calling it pre-auth would 401, and
// our shared fetch wrapper reloads the page on 401, causing an infinite loop.
export function useClaudeAuth(enabled: boolean): ClaudeAuthState {
  const [loading, setLoading] = useState(false);
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    try {
      const r = await claudeAuthApi.status();
      setAuthenticated(!!r.authenticated);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
      setAuthenticated(false);
    } finally {
      setLoading(false);
      setChecked(true);
    }
  }

  useEffect(() => {
    if (enabled) load();
    // intentionally not depending on `load` (stable reference)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  return { loading, checked, authenticated, error, refresh: load };
}
