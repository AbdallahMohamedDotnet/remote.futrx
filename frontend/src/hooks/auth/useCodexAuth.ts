import { useEffect, useState } from "preact/hooks";
import type { CodexDeviceLogin } from "../../models/auth";
import { codexAuthService } from "../../services/codexAuthService";

export interface CodexAuthState {
  loading: boolean;
  checked: boolean;
  authenticated: boolean;
  usesApiKey: boolean;
  deviceLogin?: CodexDeviceLogin;
  starting: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  startDeviceLogin: () => Promise<void>;
}

export function useCodexAuth(enabled: boolean): CodexAuthState {
  const [loading, setLoading] = useState(false);
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [usesApiKey, setUsesApiKey] = useState(false);
  const [deviceLogin, setDeviceLogin] = useState<CodexDeviceLogin | undefined>(undefined);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    try {
      const r = await codexAuthService.status();
      setAuthenticated(!!r.authenticated);
      setUsesApiKey(!!r.usesApiKey);
      setDeviceLogin(r.deviceLogin);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
      setAuthenticated(false);
      setUsesApiKey(false);
    } finally {
      setLoading(false);
      setChecked(true);
    }
  }

  async function startDeviceLogin() {
    setStarting(true);
    setError(null);
    try {
      const state = await codexAuthService.startDeviceLogin();
      setDeviceLogin(state);
      await load();
    } catch (e) {
      setError((e as Error).message);
      throw e;
    } finally {
      setStarting(false);
    }
  }

  useEffect(() => {
    if (enabled) load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  useEffect(() => {
    if (!enabled || !deviceLogin?.active) return;
    const timer = window.setInterval(() => {
      void load();
    }, 2000);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, deviceLogin?.active]);

  return {
    loading,
    checked,
    authenticated,
    usesApiKey,
    deviceLogin,
    starting,
    error,
    refresh: load,
    startDeviceLogin,
  };
}
