import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import type { AgentBrowserInfo } from "../../models/project";
import { projectService } from "../../services/projectService";
import { buildGuiUrl } from "../../components/chat/browser/browserUrls";

export type BrowserGUIStatus = "idle" | "starting" | "ready" | "error" | "stopped";

// useBrowserGUISession owns the REST lifecycle for the Agent Browser view. The
// agent-facing core can run without the pane; opening this view starts noVNC,
// while closing it stops only noVNC and leaves Chrome/CDP intact.
export function useBrowserGUISession({ projectId, enabled }: { projectId: string; enabled: boolean }) {
  const activeRef = useRef(false);
  const [status, setStatus] = useState<BrowserGUIStatus>("idle");
  const [guiUrl, setGuiUrl] = useState("");
  const [error, setError] = useState<string | null>(null);

  const applyInfo = useCallback((info: AgentBrowserInfo) => {
    if (info.view === "ready") {
      const next = buildGuiUrl(info.slug ?? "", info.port ?? 0);
      if (!next) {
        setError("Agent browser started but returned an incomplete address.");
        setStatus("error");
        setGuiUrl("");
        return;
      }
      setGuiUrl(next);
      setError(null);
      setStatus("ready");
      return;
    }
    setGuiUrl("");
    setStatus("stopped");
  }, []);

  useEffect(() => {
    if (!enabled || !projectId) {
      activeRef.current = false;
      setStatus("idle");
      setGuiUrl("");
      setError(null);
      return;
    }

    let disposed = false;
    activeRef.current = true;
    setStatus("starting");
    setGuiUrl("");
    setError(null);

    async function start() {
      try {
        const info = await projectService.startAgentBrowser(projectId);
        if (disposed) return;
        applyInfo(info);
      } catch (err) {
        if (disposed) return;
        setError(err instanceof Error ? err.message : "Failed to start the agent browser.");
        setStatus("error");
      }
    }

    void start();
    const poll = window.setInterval(() => {
      void projectService.agentBrowserStatus(projectId)
        .then((info) => {
          if (!disposed && activeRef.current) applyInfo(info);
        })
        .catch(() => {});
    }, 15000);

    return () => {
      disposed = true;
      activeRef.current = false;
      window.clearInterval(poll);
      void projectService.stopAgentBrowser(projectId, "view").catch(() => {});
    };
  }, [applyInfo, enabled, projectId]);

  const stop = useCallback(() => {
    if (!projectId) return;
    activeRef.current = false;
    setStatus("stopped");
    setGuiUrl("");
    void projectService.stopAgentBrowser(projectId)
      .then(applyInfo)
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to stop the agent browser.");
        setStatus("error");
      });
  }, [applyInfo, projectId]);

  return { status, guiUrl, error, stop };
}
