import { useEffect, useState } from "preact/hooks";
import type { CodexAuthStatus, CodexDeviceLogin } from "../../../models/auth";
import { codexAuthApi } from "../../../api/codexAuthApi";

export interface CodexAuthState {
  loading: boolean;
  checked: boolean;
  authenticated: boolean;
  usesApiKey: boolean;
  deviceLogin?: CodexDeviceLogin;
  starting: boolean;
  error: string | null;
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

  function applyStatus(status: CodexAuthStatus) {
    setAuthenticated(!!status.authenticated);
    setUsesApiKey(!!status.usesApiKey);
    setDeviceLogin(status.deviceLogin);
    setError(null);
    setLoading(false);
    setChecked(true);
  }

  async function startDeviceLogin() {
    setStarting(true);
    setError(null);
    try {
      const state = await codexAuthApi.startDeviceLogin();
      setDeviceLogin(state);
    } catch (e) {
      setError((e as Error).message);
      throw e;
    } finally {
      setStarting(false);
    }
  }

  useEffect(() => {
    if (!enabled) {
      setLoading(false);
      setChecked(false);
      setAuthenticated(false);
      setUsesApiKey(false);
      setDeviceLogin(undefined);
      setError(null);
      return;
    }

    setLoading(true);
    return codexAuthApi.subscribe(applyStatus);
  }, [enabled]);

  return {
    loading,
    checked,
    authenticated,
    usesApiKey,
    deviceLogin,
    starting,
    error,
    startDeviceLogin,
  };
}
