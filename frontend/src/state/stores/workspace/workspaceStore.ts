import type { ChatMeta } from "../../../models/chat";
import type { ProjectMeta } from "../../../models/project";
import type { WorkspaceSnapshot } from "../../../models/workspace";
import type { WorkspaceMessage } from "../../../types/workspaceApi";
import { workspaceDataProjector } from "./workspaceDataProjector.ts";

/** Opens the workspace feed and reports messages until the returned call. */
type SubscribeToWorkspace = (
  onMessage: (message: WorkspaceMessage) => void,
) => () => void;

const EMPTY: WorkspaceSnapshot = { chats: [], projects: [], loaded: false };

/**
 * The chats and projects the server is pushing, held outside the component
 * tree because the feed is one connection for the whole app rather than one
 * per mount.
 *
 * The snapshot object is rebuilt only when a message actually changes
 * something — the projector returns the same array when it does not — so a
 * subscriber that stores the snapshot re-renders on real changes and not on
 * traffic. Listeners are notified on the same condition.
 */
class WorkspaceStore {
  #snapshot: WorkspaceSnapshot = EMPTY;
  #listeners = new Set<(snapshot: WorkspaceSnapshot) => void>();
  #disconnect: (() => void) | undefined;
  readonly #subscribe: SubscribeToWorkspace;

  // The feed is injected rather than imported: it keeps this module free of the
  // api layer, which is what lets a test drive it with a hand-held feed and
  // what lets the node test runner load it at all.
  constructor(subscribe: SubscribeToWorkspace) {
    this.#subscribe = subscribe;
  }

  get snapshot(): WorkspaceSnapshot {
    return this.#snapshot;
  }

  /** Opens the feed, or closes it and clears what it delivered. Repeating a
   *  state is a no-op, so callers may drive this from an effect. */
  setConnected(connected: boolean): void {
    if (connected) {
      if (!this.#disconnect) this.#disconnect = this.#subscribe((m) => this.#apply(m));
      return;
    }
    this.#disconnect?.();
    this.#disconnect = undefined;
    this.#commit(EMPTY.chats, EMPTY.projects, false);
  }

  subscribe(listener: (snapshot: WorkspaceSnapshot) => void): () => void {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  }

  #apply(message: WorkspaceMessage): void {
    const { chats, projects, loaded } = this.#snapshot;
    switch (message.type) {
      case "workspace.snapshot":
        this.#commit(
          workspaceDataProjector.replaceChats(message.chats, chats),
          workspaceDataProjector.replaceProjects(message.projects, projects),
          true,
        );
        break;
      case "chat.upsert":
        this.#commit(workspaceDataProjector.upsertChat(chats, message.chat), projects, loaded);
        break;
      case "chat.delete":
        this.#commit(workspaceDataProjector.removeChat(chats, message.id), projects, loaded);
        break;
      case "project.upsert":
        this.#commit(chats, workspaceDataProjector.upsertProject(projects, message.project), loaded);
        break;
      case "project.delete":
        this.#commit(chats, workspaceDataProjector.removeProject(projects, message.id), loaded);
        break;
    }
  }

  #commit(chats: ChatMeta[], projects: ProjectMeta[], loaded: boolean): void {
    const current = this.#snapshot;
    if (chats === current.chats && projects === current.projects && loaded === current.loaded) {
      return;
    }
    this.#snapshot = { chats, projects, loaded };
    for (const listener of this.#listeners) listener(this.#snapshot);
  }
}

export { WorkspaceStore };
