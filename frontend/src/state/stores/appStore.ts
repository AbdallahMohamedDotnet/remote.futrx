import { createStore, type StoreApi } from "zustand/vanilla";

export interface AppStoreShape<State, Actions> {
  state: State;
  actions: Actions;
}

export type AppStore<State, Actions> = StoreApi<AppStoreShape<State, Actions>>;

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
  return createStore<AppStoreShape<State, Actions>>()((set, get) => {
    const access: StoreAccess<State> = {
      getState: () => get().state,
      setState: (update) => set((store) => {
        const next = isStateUpdater(update) ? update(store.state) : update;
        if (Object.is(next, store.state)) return store;
        return { state: { ...store.state, ...next } };
      }),
    };

    return {
      state: initialState,
      actions: createActions(access),
    };
  });
}

function isStateUpdater<State>(update: StateUpdate<State>): update is StateUpdater<State> {
  return typeof update === "function";
}
