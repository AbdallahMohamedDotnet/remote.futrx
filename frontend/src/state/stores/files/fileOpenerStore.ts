import type { FileOpener } from "../../../models/files.ts";

class FileOpenerStore {
  private readonly openers = new Map<string, { open: FileOpener; projectIds: string[] }>();
  private readonly listeners = new Set<() => void>();

  private notify(): void {
    for (const listener of this.listeners) listener();
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  }
  register(applicationId: string, projectIds: string[], open: FileOpener): () => void {
    const entry = { open, projectIds };
    this.openers.set(applicationId, entry);
    this.notify();
    return () => {
      if (this.openers.get(applicationId) === entry) {
        this.openers.delete(applicationId);
        this.notify();
      }
    };
  }
  setProjects(applicationId: string, projectIds: string[]): void {
    const entry = this.openers.get(applicationId);
    if (entry && (entry.projectIds.length !== projectIds.length ||
      entry.projectIds.some((id) => !projectIds.includes(id)))) {
      entry.projectIds = projectIds;
      this.notify();
    }
  }
  remove(applicationId: string): void {
    if (this.openers.delete(applicationId)) this.notify();
  }
  canOpen(projectId?: string): boolean {
    return Boolean(projectId && [...this.openers.values()].some((entry) => entry.projectIds.includes(projectId)));
  }
  *forProject(projectId?: string): IterableIterator<FileOpener> {
    if (!projectId) return;
    for (const entry of this.openers.values()) {
      if (entry.projectIds.includes(projectId)) yield (request) => entry.open(request);
    }
  }
}

export const fileOpenerStore = new FileOpenerStore();
