import { useEffect, useRef, useState } from "preact/hooks";

interface UsePollOptions<T> {
  equals?: (a: T, b: T) => boolean;
  pauseWhenHidden?: boolean;
}

// Poll an async function on an interval. Returns latest value + error +
// a refresh() trigger. Cancels in-flight on unmount.
export function usePoll<T>(
  fn: () => Promise<T>,
  intervalMs: number,
  initial: T,
  options: UsePollOptions<T> = {}
): { value: T; error: Error | null; refresh: () => Promise<void> } {
  const [value, setValue] = useState<T>(initial);
  const [error, setError] = useState<Error | null>(null);
  const aliveRef = useRef(true);
  const tickRef = useRef(0);
  const fnRef = useRef(fn);
  const valueRef = useRef(initial);
  const optionsRef = useRef(options);
  fnRef.current = fn;
  optionsRef.current = options;

  const refresh = async (force = true) => {
    if (!force && shouldPause(optionsRef.current)) return;
    const ticket = ++tickRef.current;
    try {
      const nextValue = await fnRef.current();
      if (aliveRef.current && tickRef.current === ticket) {
        const equals = optionsRef.current.equals ?? Object.is;
        if (!equals(valueRef.current, nextValue)) {
          valueRef.current = nextValue;
          setValue(nextValue);
        }
        setError((current) => current === null ? current : null);
      }
    } catch (error) {
      if (aliveRef.current && tickRef.current === ticket) {
        const nextError = error instanceof Error ? error : new Error(String(error));
        setError((current) => current?.message === nextError.message ? current : nextError);
      }
    }
  };

  useEffect(() => {
    aliveRef.current = true;
    refresh();
    const id = window.setInterval(() => refresh(false), intervalMs);
    const onVisible = () => {
      if (!document.hidden) refresh();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      aliveRef.current = false;
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVisible);
    };
    // intentionally only re-run on intervalMs change
  }, [intervalMs]);

  return { value, error, refresh: () => refresh() };
}

function shouldPause<T>(options: UsePollOptions<T>): boolean {
  return options.pauseWhenHidden !== false && typeof document !== "undefined" && document.hidden;
}
