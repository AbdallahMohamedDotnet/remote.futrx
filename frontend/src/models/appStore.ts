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
  subscribe: (listener: AppStoreListener<State, Actions>) => () => void;
}

export type StateUpdater<State> = (state: State) => Partial<State> | State;
export type StateUpdate<State> = Partial<State> | StateUpdater<State>;

export interface StoreAccess<State> {
  getState: () => State;
  setState: (update: StateUpdate<State>) => void;
}
