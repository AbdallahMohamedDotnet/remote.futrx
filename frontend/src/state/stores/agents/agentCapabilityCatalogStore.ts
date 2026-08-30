import type {
  AgentCapabilitiesCatalog,
  AgentCapabilityCatalogSnapshot,
} from "../../../models/agentCapabilities";
import { capabilitiesApi } from "../../../api/agents/capabilitiesApi.ts";
import { Listeners } from "../listeners.ts";

/** A scope some part of the app is currently watching, and its listeners. */
interface ObservedScope {
  /** Normalized, matching the user half of the scope's key. */
  userId: string;
  /** Empty for the host scope, matching the project half of the key. */
  projectId: string;
  listeners: Listeners;
}

type CatalogRequester = (
  projectId?: string,
  options?: { refresh?: boolean },
) => Promise<AgentCapabilitiesCatalog>;

// This store keeps the last response for each normalized user and host/project
// scope only for the lifetime of the open application. The process-local
// backend cache owns freshness across browsers and devices. Retaining the last
// frontend response avoids a visual reset while a backend lookup or refresh is
// in flight; in-flight requests for the same frontend scope are coalesced.
export class AgentCapabilityCatalogStore {
  private readonly catalogs = new Map<string, AgentCapabilitiesCatalog>();
  private readonly errors = new Map<string, string>();
  private readonly inFlight = new Map<string, Promise<AgentCapabilitiesCatalog>>();
  private readonly observed = new Map<string, ObservedScope>();
  private readonly request: CatalogRequester;

  constructor(request: CatalogRequester) {
    this.request = request;
  }

  read(userId: string, projectId?: string): AgentCapabilityCatalogSnapshot {
    const key = catalogKey(userId, projectId);
    const catalog = this.catalogs.get(key) ?? null;
    const refreshing = this.inFlight.has(key);
    return {
      catalog,
      loading: refreshing && !catalog,
      refreshing,
      error: this.errors.get(key) ?? "",
    };
  }

  subscribe(userId: string, projectId: string | undefined, listener: () => void): () => void {
    const key = catalogKey(userId, projectId);
    const scope = this.observed.get(key) ?? {
      userId: normalizeUserId(userId),
      projectId: projectId || "",
      listeners: new Listeners(),
    };
    const remove = scope.listeners.add(listener);
    this.observed.set(key, scope);
    return () => {
      remove();
      if (scope.listeners.size === 0) this.observed.delete(key);
    };
  }

  load(
    userId: string,
    projectId?: string,
    options: { force?: boolean } = {},
  ): Promise<AgentCapabilitiesCatalog> {
    const key = catalogKey(userId, projectId);
    const existing = this.inFlight.get(key);
    if (existing) return existing;

    this.errors.delete(key);
    const running = (async () => {
      try {
        const catalog = await this.request(projectId, { refresh: !!options.force });
        this.catalogs.set(key, catalog);
        this.errors.delete(key);
        return catalog;
      } catch (cause) {
        this.errors.set(key, errorMessage(cause));
        throw cause;
      } finally {
        this.inFlight.delete(key);
        this.notify(key);
      }
    })();

    this.inFlight.set(key, running);
    this.notify(key);
    return running;
  }

  invalidateUser(userId: string): void {
    // A managed host-auth change can alter every catalog. Request a
    // force-refresh for scopes currently observed by this browser; an existing
    // request for the same scope remains coalesced.
    const normalizedUser = normalizeUserId(userId);
    for (const scope of this.observed.values()) {
      if (scope.userId !== normalizedUser) continue;
      void this.load(scope.userId, scope.projectId || undefined, { force: true })
        .catch(() => undefined);
    }
  }

  removeProject(userId: string, projectId: string): void {
    const key = catalogKey(userId, projectId);
    this.catalogs.delete(key);
    this.errors.delete(key);
    this.notify(key);
  }

  private notify(key: string): void {
    this.observed.get(key)?.listeners.emit();
  }
}

function catalogKey(userId: string, projectId?: string): string {
  return JSON.stringify([normalizeUserId(userId), projectId || ""]);
}

function normalizeUserId(userId: string): string {
  return userId.trim().toLowerCase() || "anonymous";
}

function errorMessage(cause: unknown): string {
  return cause instanceof Error && cause.message
    ? cause.message
    : "Could not load agent capabilities";
}

// Every caller observes one instance so the retained responses above are shared
// across the application rather than rebuilt per consumer.
export const agentCapabilityCatalogStore = new AgentCapabilityCatalogStore(
  capabilitiesApi.list,
);
