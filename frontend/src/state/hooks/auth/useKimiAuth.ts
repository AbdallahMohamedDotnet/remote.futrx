import { useEffect, useState } from "preact/hooks";
import type { KimiAuthStatus, KimiDeviceLogin } from "../../../models/auth";
import { kimiAuthApi } from "../../../api/agents/auth/kimiAuthApi";

export interface KimiAuthState {
  loading: boolean;
  checked: boolean;
  authenticated: boolean;
  deviceLogin?: KimiDeviceLogin;
  starting: boolean;
  error: string | null;
  startDeviceLogin: () => Promise<void>;
}

export function useKimiAuth(enabled: boolean): KimiAuthState {
  const [loading, setLoading] = useState(false);
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [deviceLogin, setDeviceLogin] = useState<KimiDeviceLogin | undefined>(undefined);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function applyStatus(status: KimiAuthStatus) {
    setAuthenticated(!!status.authenticated);
    setDeviceLogin(status.deviceLogin);
    setError(null);
    setLoading(false);
    setChecked(true);
  }

  async function startDeviceLogin() {
    setStarting(true);
    setError(null);
    try {
      const state = await kimiAuthApi.startDeviceLogin();
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
      setDeviceLogin(undefined);
      setError(null);
      return;
    }

    setLoading(true);
    return kimiAuthApi.subscribe(applyStatus);
  }, [enabled]);

  return {
    loading,
    checked,
    authenticated,
    deviceLogin,
    starting,
    error,
    startDeviceLogin,
  };
}
