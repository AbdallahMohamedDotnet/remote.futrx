import { useEffect, useRef, useState } from "preact/hooks";
import { chatApi } from "../../../api/chatApi";
import type { ChatStatus } from "../../../models/chat";

// Whether the chat's workspace contains at least one git repository. Drives
// the History button: it only appears once a repo is confirmed. Re-checked
// when a run finishes, since the agent may have just created or cloned one.
export function useWorkspaceGitRepos({ chatId, status }: { chatId: string; status: ChatStatus }) {
  const [hasRepos, setHasRepos] = useState(false);
  const checkedRunRef = useRef(false);

  useEffect(() => {
    setHasRepos(false);
    checkedRunRef.current = false;
  }, [chatId]);

  useEffect(() => {
    // One check when the chat becomes ready, renewed after each run so a repo
    // created mid-session surfaces without a reload. Once repos are known to
    // exist, stop re-checking — repos disappearing mid-session is not a case
    // worth polling for.
    if (status === "streaming") {
      checkedRunRef.current = false;
      return;
    }
    if (status !== "ready" || hasRepos || checkedRunRef.current) return;
    checkedRunRef.current = true;

    let cancelled = false;
    (async () => {
      try {
        const response = await chatApi.fetchHistoryRepos(chatId);
        if (!cancelled && (response.repos || []).length > 0) setHasRepos(true);
      } catch {
        // Workspace may be stopped or unreachable — keep the button hidden.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [chatId, status, hasRepos]);

  return { hasRepos };
}
