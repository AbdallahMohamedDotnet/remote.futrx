import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import type { AgentBrowserInfo, AgentBrowserStatus } from "../../models/project";
import { projectService } from "../../services/projectService";

export type { AgentBrowserStatus };

const pollIntervalMs = 1500;

// useAgentBrowserSession asks the backend to bring up the in-container Agent
// Browser and tracks its status over project REST endpoints. Pixels do NOT
// flow here: once ready, the noVNC view loads as an iframe from the dev-URL
// proxy. Closing the drawer does not stop the browser; the explicit stop does.
export function useAgentBrowserSession({ projectId, enabled }: { projectId: string; enabled: boolean }) {
  const [status, setStatus] = useState<AgentBrowserStatus>("idle");
  const [guiUrl, setGuiUrl] = useState("");
  const [error, setError] = useState<string | null>(null);
  const requestRef = useRef(0);
  const mountedRef = useRef(true);

  useEffect(() => {
    return () => {
      mountedRef.current = false;
      requestRef.current++;
    };
  }, []);

  const applyInfo = useCallback((info: AgentBrowserInfo): boolean => {
    if (info.status === "ready") {
      if (!info.url) {
        setGuiUrl("");
        setError("Agent browser started but returned an incomplete address.");
        setStatus("error");
        return false;
      }
      setError(null);
      setGuiUrl(info.url);
      setStatus("ready");
      return false;
    }
    if (info.status === "error") {
      setGuiUrl("");
      setError(info.error || "Failed to start the agent browser.");
      setStatus("error");
      return false;
    }
    setError(null);
    setGuiUrl("");
    setStatus(info.status);
    return info.status === "starting";
  }, []);

  useEffect(() => {
    const requestId = ++requestRef.current;
    if (!enabled || !projectId) {
      setStatus("idle");
      setGuiUrl("");
      setError(null);
      return;
    }

    let disposed = false;
    let pollTimer: number | undefined;
    setStatus("starting");
    setGuiUrl("");
    setError(null);

    const isCurrent = () => mountedRef.current && !disposed && requestRef.current === requestId;

    async function pollStatus() {
      try {
        const info = await projectService.agentBrowserStatus(projectId);
        if (!isCurrent()) return;
        if (applyInfo(info)) pollTimer = window.setTimeout(pollStatus, pollIntervalMs);
      } catch (err) {
        if (!isCurrent()) return;
        setError((err as Error).message || "Failed to check the agent browser.");
        setStatus("error");
      }
    }

    projectService.startAgentBrowser(projectId)
      .then((info) => {
        if (!isCurrent()) return;
        if (applyInfo(info)) pollTimer = window.setTimeout(pollStatus, pollIntervalMs);
      })
      .catch((err) => {
        if (!isCurrent()) return;
        setError((err as Error).message || "Failed to start the agent browser.");
        setStatus("error");
      });

    return () => {
      disposed = true;
      requestRef.current++;
      if (pollTimer !== undefined) window.clearTimeout(pollTimer);
    };
  }, [projectId, enabled, applyInfo]);

  const stop = useCallback(() => {
    if (!projectId) return;
    const requestId = ++requestRef.current;
    setStatus("starting");
    setGuiUrl("");
    setError(null);
    projectService.stopAgentBrowser(projectId)
      .then(() => {
        if (!mountedRef.current || requestRef.current !== requestId) return;
        setGuiUrl("");
        setStatus("stopped");
      })
      .catch((err) => {
        if (!mountedRef.current || requestRef.current !== requestId) return;
        setError((err as Error).message || "Failed to stop the agent browser.");
        setStatus("error");
      });
  }, [projectId]);

  return { status, guiUrl, error, stop };
}
