import { useSyncExternalStore } from "preact/compat";
import type { AppStore, AppStoreShape } from "../stores/appStore.ts";

export function useAppStore<State, Actions, Selection>(
  store: AppStore<State, Actions>,
  selector: (store: AppStoreShape<State, Actions>) => Selection,
): Selection {
  return useSyncExternalStore(
    store.subscribe,
    () => selector(store.getState()),
  );
}
