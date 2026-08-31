import { useLayoutEffect, useRef, useState } from "preact/hooks";
import type { StoreApi } from "zustand/vanilla";

type StoreSelector<State, Selection> = (state: State) => Selection;
type Equality<Selection> = (left: Selection, right: Selection) => boolean;

interface CurrentSelection<State, Selection> {
  store: StoreApi<State>;
  value: Selection;
}

/**
 * Subscribes a Preact component to a vanilla Zustand store without loading
 * Zustand's React binding (and therefore preact/compat) into the render tree.
 */
export function useZustandStore<State, Selection>(
  store: StoreApi<State>,
  selector: StoreSelector<State, Selection>,
  equal: Equality<Selection> = Object.is,
): Selection {
  const selectorRef = useRef(selector);
  const equalityRef = useRef(equal);
  const currentRef = useRef<CurrentSelection<State, Selection> | null>(null);
  const [, setRevision] = useState(0);

  selectorRef.current = selector;
  equalityRef.current = equal;

  const selected = selector(store.getState());
  const current = currentRef.current;
  if (!current || current.store !== store || !equal(current.value, selected)) {
    currentRef.current = { store, value: selected };
  }

  useLayoutEffect(() => {
    const update = (state: State) => {
      if (currentRef.current?.store !== store) return;
      const next = selectorRef.current(state);
      if (equalityRef.current(currentRef.current.value, next)) return;
      currentRef.current = { store, value: next };
      setRevision((revision) => revision + 1);
    };

    // Close the render-to-subscribe race before the browser can paint.
    update(store.getState());
    return store.subscribe(update);
  }, [store]);

  return currentRef.current!.value;
}
