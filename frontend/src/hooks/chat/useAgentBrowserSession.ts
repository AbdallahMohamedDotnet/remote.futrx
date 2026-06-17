import { useCallback, useEffect, useState } from "preact/hooks";
import type { AgentBrowserStatus } from "../../models/project";
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

  useEffect(() => {
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

    async function pollStatus() {
      try {
        const info = await projectService.agentBrowserStatus(projectId);
        if (disposed) return;
        if (info.status === "ready") {
          if (!info.url) {
            setError("Agent browser started but returned an incomplete address.");
            setStatus("error");
            return;
          }
          setGuiUrl(info.url);
          setStatus("ready");
          return;
        }
        setGuiUrl("");
        setStatus((current) => (current === "starting" ? "starting" : "stopped"));
        pollTimer = window.setTimeout(pollStatus, pollIntervalMs);
      } catch (err) {
        if (disposed) return;
        setError((err as Error).message || "Failed to check the agent browser.");
        setStatus("error");
      }
    }

    projectService.startAgentBrowser(projectId)
      .then((info) => {
        if (disposed) return;
        if (info.status === "ready" && info.url) {
          setGuiUrl(info.url);
          setStatus("ready");
          return;
        }
        pollTimer = window.setTimeout(pollStatus, pollIntervalMs);
      })
      .catch((err) => {
        if (disposed) return;
        setError((err as Error).message || "Failed to start the agent browser.");
        setStatus("error");
      });

    return () => {
      disposed = true;
      if (pollTimer !== undefined) window.clearTimeout(pollTimer);
    };
  }, [projectId, enabled]);

  const stop = useCallback(() => {
    if (!projectId) return;
    setStatus("starting");
    projectService.stopAgentBrowser(projectId)
      .then(() => {
        setGuiUrl("");
        setStatus("stopped");
      })
      .catch((err) => {
        setError((err as Error).message || "Failed to stop the agent browser.");
        setStatus("error");
      });
  }, [projectId]);

  const refresh = useCallback(() => {
    if (!projectId) return;
    projectService.agentBrowserStatus(projectId)
      .then((info) => {
        if (info.status === "ready" && info.url) {
          setGuiUrl(info.url);
          setStatus("ready");
        } else {
          setGuiUrl("");
          setStatus(info.status);
        }
      })
      .catch((err) => {
        setError((err as Error).message || "Failed to check the agent browser.");
        setStatus("error");
      });
  }, [projectId]);

  return { status, guiUrl, error, stop, refresh };
}
