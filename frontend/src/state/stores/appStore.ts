import type {
  AppStore,
  AppStoreListener,
  AppStoreShape,
  StateUpdate,
  StateUpdater,
  StoreAccess,
} from "../../models/appStore.ts";

/**
 * Creates the one supported store shape: domain state and domain actions are
 * separate, while actions can only read or update the state half.
 */
export function createAppStore<State extends object, Actions extends object>(
  initialState: State,
  createActions: (access: StoreAccess<State>) => Actions,
): AppStore<State, Actions> {
  const listeners = new Set<AppStoreListener<State, Actions>>();
  let state = initialState;
  let store: AppStoreShape<State, Actions>;

  const access: StoreAccess<State> = {
    getState: () => state,
    setState: (update) => {
      const next = isStateUpdater(update) ? update(state) : update;
      if (Object.is(next, state)) return;

      const previousStore = store;
      state = { ...state, ...next };
      store = { state, actions: store.actions };
      listeners.forEach((listener) => listener(store, previousStore));
    },
  };

  const actions = createActions(access);
  store = { state, actions };

  return {
    getState: () => store,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

function isStateUpdater<State>(update: StateUpdate<State>): update is StateUpdater<State> {
  return typeof update === "function";
}
