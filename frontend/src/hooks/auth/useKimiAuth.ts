import { useEffect, useState } from "preact/hooks";
import { kimiAuthWebSocketUrl } from "../../transport/websocket";
import type { KimiAuthStatus, KimiDeviceLogin } from "../../models/auth";
import { kimiAuthService } from "../../api/kimiAuthService";

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
      const state = await kimiAuthService.startDeviceLogin();
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

    let stopped = false;
    let attempt = 0;
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    setLoading(true);

    function scheduleReconnect() {
      if (stopped) return;
      const delay = Math.min(5000, 400 * 2 ** attempt);
      attempt++;
      reconnectTimer = setTimeout(connect, delay);
    }

    function connect() {
      if (stopped) return;
      socket = new WebSocket(kimiAuthWebSocketUrl());

      socket.onopen = () => {
        attempt = 0;
      };

      socket.onmessage = (event) => {
        try {
          applyStatus(JSON.parse(event.data) as KimiAuthStatus);
        } catch {}
      };

      socket.onclose = () => {
        socket = null;
        scheduleReconnect();
      };

      socket.onerror = () => {
        try { socket?.close(); } catch {}
      };
    }

    connect();

    return () => {
      stopped = true;
      if (reconnectTimer) window.clearTimeout(reconnectTimer);
      try { socket?.close(); } catch {}
    };
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
