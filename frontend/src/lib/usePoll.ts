import { useEffect, useRef, useState } from "preact/hooks";

// Poll an async function on an interval. Returns latest value + error +
// a refresh() trigger. Cancels in-flight on unmount.
export function usePoll<T>(
  fn: () => Promise<T>,
  intervalMs: number,
  initial: T
): { value: T; error: Error | null; refresh: () => void } {
  const [value, setValue] = useState<T>(initial);
  const [error, setError] = useState<Error | null>(null);
  const aliveRef = useRef(true);
  const tickRef = useRef(0);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const refresh = () => {
    const ticket = ++tickRef.current;
    fnRef.current()
      .then((v) => {
        if (aliveRef.current && tickRef.current === ticket) {
          setValue(v);
          setError(null);
        }
      })
      .catch((e) => {
        if (aliveRef.current && tickRef.current === ticket) {
          setError(e instanceof Error ? e : new Error(String(e)));
        }
      });
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
