import type { FileOpenRequest } from "../../models/files.ts";

type FileOpener = (request: FileOpenRequest) => string | null;

const openers = new Map<string, { open: FileOpener; projectIds: string[] }>();
const listeners = new Set<() => void>();
const notify = () => { for (const listener of listeners) listener(); };

export const fileOpenerRegistry = {
  subscribe(listener: () => void): () => void {
    listeners.add(listener);
    return () => { listeners.delete(listener); };
  },
  register(applicationId: string, projectIds: string[], open: FileOpener): () => void {
    const entry = { open, projectIds };
    openers.set(applicationId, entry);
    notify();
    return () => {
      if (openers.get(applicationId) === entry) {
        openers.delete(applicationId);
        notify();
      }
    };
  },
  setProjects(applicationId: string, projectIds: string[]): void {
    const entry = openers.get(applicationId);
    if (entry && (entry.projectIds.length !== projectIds.length ||
      entry.projectIds.some((id) => !projectIds.includes(id)))) {
      entry.projectIds = projectIds;
      notify();
    }
  },
  remove(applicationId: string): void {
    if (openers.delete(applicationId)) notify();
  },
  canOpen(projectId?: string): boolean {
    return Boolean(projectId && [...openers.values()].some((entry) => entry.projectIds.includes(projectId)));
  },
  url(projectId: string | undefined, request: FileOpenRequest): string | null {
    if (!projectId) return null;
    for (const entry of openers.values()) {
      if (!entry.projectIds.includes(projectId)) continue;
      try {
        const url = entry.open(request);
        if (url) return url;
      } catch (error) {
        console.error("[extensions] file opener failed", error);
      }
    }
    return null;
  },
};
