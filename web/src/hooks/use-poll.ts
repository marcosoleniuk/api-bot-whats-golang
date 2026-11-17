"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface Result<T> {
  key: string;
  data?: T;
  error?: Error;
}

/**
 * Busca dados com atualização periódica opcional. Mudar `deps` descarta o
 * resultado anterior (loading volta a ser true) e busca de novo.
 */
export function usePoll<T>(fn: () => Promise<T>, intervalMs = 0, deps: unknown[] = []) {
  const depsKey = JSON.stringify(deps);
  const [res, setRes] = useState<Result<T>>({ key: "" });
  const fnRef = useRef(fn);
  const runRef = useRef<() => Promise<void>>(async () => {});

  useEffect(() => {
    fnRef.current = fn;
  });

  useEffect(() => {
    let alive = true;
    const run = async () => {
      try {
        const data = await fnRef.current();
        if (alive) setRes({ key: depsKey, data });
      } catch (e) {
        if (alive) {
          setRes((p) => ({
            key: depsKey,
            data: p.key === depsKey ? p.data : undefined,
            error: e instanceof Error ? e : new Error(String(e)),
          }));
        }
      }
    };
    runRef.current = run;
    void run();
    const id = intervalMs ? setInterval(() => void run(), intervalMs) : undefined;
    return () => {
      alive = false;
      if (id) clearInterval(id);
    };
  }, [depsKey, intervalMs]);

  const reload = useCallback(() => runRef.current(), []);
  const current = res.key === depsKey;

  return {
    data: current ? res.data : undefined,
    error: current ? res.error : undefined,
    loading: !current,
    reload,
  };
}
