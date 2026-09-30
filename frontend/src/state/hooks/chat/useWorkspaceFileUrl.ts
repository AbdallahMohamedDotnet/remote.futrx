import { builtinWorkspaceFileUrl } from "../../../ui/chat/ideLinks.ts";
import { useCallback, useEffect, useState } from "preact/hooks";
import { fileOpenerRegistry } from "../../../services/files/fileOpenerRegistry.ts";
import type { FileOpenRequest } from "../../../models/files.ts";
import { extensionStore } from "../../stores/extensions/extensionStore.ts";

// Refresh rendered links when an installed application changes its opener.
export function useWorkspaceFileUrl(): (request: FileOpenRequest) => string | null {
  const [openerVersion, setVersion] = useState(0);
  useEffect(() => {
    const refresh = () => setVersion((version) => version + 1);
    const unsubscribeOpener = fileOpenerRegistry.subscribe(refresh);
    const unsubscribeProject = extensionStore.subscribe((current, previous) => {
      if (current.activeProjectId !== previous.activeProjectId) refresh();
    });
    return () => {
      unsubscribeOpener();
      unsubscribeProject();
    };
  }, []);
  return useCallback(
    (request: FileOpenRequest) => fileOpenerRegistry.url(extensionStore.getState().activeProjectId ?? undefined, request)
      ?? builtinWorkspaceFileUrl(request),
    [openerVersion],
  );
}
