"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import type { SessionsData, WaSession } from "@/lib/types";
import { usePoll } from "@/hooks/use-poll";

interface Ctx {
  tenantKey: string;
  sessions: WaSession[];
  loading: boolean;
  error?: Error;
  reload: () => Promise<void>;
  /** Chave da sessão atualmente selecionada nos formulários. */
  selectedKey: string;
  setSelectedKey: (k: string) => void;
  selected?: WaSession;
}

const SessionCtx = createContext<Ctx | null>(null);
const STORAGE_KEY = "wb_selected_session";

export function SessionProvider({ tenantKey, children }: { tenantKey: string; children: React.ReactNode }) {
  const { data, error, loading, reload } = usePoll(() => api<SessionsData>("whatsapp/sessions"), 15_000);
  const sessions = useMemo(() => data?.sessions ?? [], [data]);
  const [selectedKey, setSelectedKeyState] = useState("");

  useEffect(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved) setSelectedKeyState(saved);
    } catch {}
  }, []);

  // Garante uma seleção válida.
  useEffect(() => {
    if (!sessions.length) return;
    if (!sessions.some((s) => s.whatsapp_session_key === selectedKey)) {
      const first = sessions.find((s) => s.status === "connected") ?? sessions[0];
      setSelectedKeyState(first.whatsapp_session_key);
    }
  }, [sessions, selectedKey]);

  const setSelectedKey = useCallback((k: string) => {
    setSelectedKeyState(k);
    try {
      localStorage.setItem(STORAGE_KEY, k);
    } catch {}
  }, []);

  const value: Ctx = {
    tenantKey,
    sessions,
    loading,
    error,
    reload: async () => {
      await reload();
    },
    selectedKey,
    setSelectedKey,
    selected: sessions.find((s) => s.whatsapp_session_key === selectedKey),
  };

  return <SessionCtx.Provider value={value}>{children}</SessionCtx.Provider>;
}

export function useSessions() {
  const ctx = useContext(SessionCtx);
  if (!ctx) throw new Error("useSessions fora do SessionProvider");
  return ctx;
}
