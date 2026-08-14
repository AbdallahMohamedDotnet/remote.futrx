import { useEffect, useState } from "preact/hooks";
import { capabilitiesApi } from "../../../api/agents/capabilitiesApi";
import type { AgentCapabilitiesCatalog } from "../../../models/agentCapabilities";

export function useAgentCapabilities(projectId?: string) {
  const [catalog, setCatalog] = useState<AgentCapabilitiesCatalog | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setCatalog(null);
    setLoading(true);
    setError("");
    capabilitiesApi.list(projectId)
      .then((nextCatalog) => {
        if (!cancelled) setCatalog(nextCatalog);
      })
      .catch((cause) => {
        if (!cancelled) {
          setCatalog(null);
          setError((cause as Error).message || "Could not load agent capabilities");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  return { catalog, loading, error };
}
