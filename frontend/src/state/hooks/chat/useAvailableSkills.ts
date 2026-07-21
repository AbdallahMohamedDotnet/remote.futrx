import { useEffect, useState } from "preact/hooks";
import { skillApi } from "../../../api/skillApi";
import type { ChatProvider } from "../../../models/chat";
import type { RegisteredSkill } from "../../../models/skill";

export function useAvailableSkills(provider: ChatProvider, projectId?: string) {
  const [skills, setSkills] = useState<RegisteredSkill[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setError("");
    setLoading(true);
    skillApi.list(provider, projectId)
      .then((items) => {
        if (!cancelled) setSkills(items);
      })
      .catch((loadError) => {
        if (!cancelled) {
          setSkills([]);
          setError((loadError as Error).message || "Could not load skills");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [provider, projectId]);

  return { skills, loading, error };
}
