"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/** Busca dados, com atualização periódica opcional. `deps` reinicia a busca. */
export function usePoll<T>(fn: () => Promise<T>, intervalMs = 0, deps: unknown[] = []) {
  const [data, setData] = useState<T | undefined>();
  const [error, setError] = useState<Error | undefined>();
  const [loading, setLoading] = useState(true);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      setData(await fnRef.current());
      setError(undefined);
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    if (!intervalMs) return;
    const id = setInterval(() => void load(true), intervalMs);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [load, intervalMs, ...deps]);

  return { data, error, loading, reload: () => load(), setData };
}
