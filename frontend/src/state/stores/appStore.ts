export interface AppStoreShape<State, Actions> {
  readonly state: State;
  readonly actions: Actions;
}

export type AppStoreListener<State, Actions> = (
  store: AppStoreShape<State, Actions>,
  previousStore: AppStoreShape<State, Actions>,
) => void;

export interface AppStore<State, Actions> {
  getState: () => AppStoreShape<State, Actions>;
  getInitialState: () => AppStoreShape<State, Actions>;
  subscribe: (listener: AppStoreListener<State, Actions>) => () => void;
}

type StateUpdater<State> = (state: State) => Partial<State> | State;
type StateUpdate<State> = Partial<State> | StateUpdater<State>;

interface StoreAccess<State> {
  getState: () => State;
  setState: (update: StateUpdate<State>) => void;
}

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
  const initialStore: AppStoreShape<State, Actions> = { state, actions };
  store = initialStore;

  return {
    getState: () => store,
    getInitialState: () => initialStore,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

function isStateUpdater<State>(update: StateUpdate<State>): update is StateUpdater<State> {
  return typeof update === "function";
}
