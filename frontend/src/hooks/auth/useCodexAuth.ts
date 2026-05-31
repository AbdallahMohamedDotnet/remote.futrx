import { useEffect, useState } from "preact/hooks";
import { codexAuthService } from "../../services/codexAuthService";

export interface CodexAuthState {
  loading: boolean;
  checked: boolean;
  authenticated: boolean;
  saving: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  loginWithAPIKey: (apiKey: string) => Promise<void>;
}

export function useCodexAuth(enabled: boolean): CodexAuthState {
  const [loading, setLoading] = useState(false);
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    try {
      const r = await codexAuthService.status();
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

  async function loginWithAPIKey(apiKey: string) {
    setSaving(true);
    setError(null);
    try {
      await codexAuthService.loginWithAPIKey(apiKey);
      await load();
    } catch (e) {
      setError((e as Error).message);
      throw e;
    } finally {
      setSaving(false);
    }
  }

  useEffect(() => {
    if (enabled) load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  return { loading, checked, authenticated, saving, error, refresh: load, loginWithAPIKey };
}
