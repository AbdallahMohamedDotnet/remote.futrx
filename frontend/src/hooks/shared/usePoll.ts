import { useEffect, useRef, useState } from "preact/hooks";

// Poll an async function on an interval. Returns latest value + error +
// a refresh() trigger. Cancels in-flight on unmount.
export function usePoll<T>(
  fn: () => Promise<T>,
  intervalMs: number,
  initial: T
): { value: T; error: Error | null; refresh: () => Promise<void> } {
  const [value, setValue] = useState<T>(initial);
  const [error, setError] = useState<Error | null>(null);
  const aliveRef = useRef(true);
  const tickRef = useRef(0);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const refresh = async () => {
    const ticket = ++tickRef.current;
    try {
      const value = await fnRef.current();
      if (aliveRef.current && tickRef.current === ticket) {
        setValue(value);
        setError(null);
      }
    } catch (error) {
      if (aliveRef.current && tickRef.current === ticket) {
        setError(error instanceof Error ? error : new Error(String(error)));
      }
    }
  };

  useEffect(() => {
    aliveRef.current = true;
    refresh();
    const id = window.setInterval(refresh, intervalMs);
    return () => {
      aliveRef.current = false;
      window.clearInterval(id);
    };
    // intentionally only re-run on intervalMs change
  }, [intervalMs]);

  return { value, error, refresh };
}
