import { useEffect, useState } from "preact/hooks";
import { claudeAuthWebSocketUrl } from "../../../transport/websocket";
import type { ClaudeAuthStatus, ClaudeLoginState } from "../../../models/auth";
import { claudeAuthApi } from "../../../api/claudeAuthApi";

export interface ClaudeAuthState {
  loading: boolean;
  // true once we've received at least one status frame. Lets the gating UI
  // show a spinner BETWEEN the moment we start listening (googleOk became
  // true) and the moment the first frame lands.
  checked: boolean;
  authenticated: boolean;
  login?: ClaudeLoginState;
  error: string | null;
  refresh: () => Promise<void>;
}

// Subscribes to /ws/claude/auth-status for live auth state, matching how
// useCodexAuth consumes Codex. Used by AuthContext to gate the workspace on
// the claude CLI being authenticated against Anthropic on the server, and by
// the settings pill so an admin re-login reflects instantly everywhere.
//
// `enabled` MUST be false until Google auth is confirmed — the auth middleware
// wraps every route including this WS, so connecting pre-auth is pointless.
export function useClaudeAuth(enabled: boolean): ClaudeAuthState {
  const [loading, setLoading] = useState(false);
  const [checked, setChecked] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [login, setLogin] = useState<ClaudeLoginState | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);

  function applyStatus(status: ClaudeAuthStatus) {
    setAuthenticated(!!status.authenticated);
    setLogin(status.login);
    setError(null);
    setLoading(false);
    setChecked(true);
  }

  // Kept for backward compat with callers that pass it as an onDone handler.
  // The WS keeps status live, so this is a best-effort one-shot resync.
  async function refresh() {
    try {
      applyStatus(await claudeAuthApi.status());
    } catch (e) {
      setError((e as Error).message);
    }
  }

  useEffect(() => {
    if (!enabled) {
      setLoading(false);
      setChecked(false);
      setAuthenticated(false);
      setLogin(undefined);
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
      socket = new WebSocket(claudeAuthWebSocketUrl());

      socket.onopen = () => {
        attempt = 0;
      };

      socket.onmessage = (event) => {
        try {
          applyStatus(JSON.parse(event.data) as ClaudeAuthStatus);
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

  return { loading, checked, authenticated, login, error, refresh };
}
