"use client";

import { createContext, useCallback, useContext, useMemo, useState } from "react";
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
  // O provider só monta no cliente (depois de /api/auth/me), então ler localStorage aqui é seguro.
  const [preferredKey, setSelectedKeyState] = useState(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) ?? "";
    } catch {
      return "";
    }
  });

  // Seleção efetiva: a preferida, se existir; senão a primeira conectada (ou a primeira).
  const selectedKey = useMemo(() => {
    if (sessions.some((s) => s.whatsapp_session_key === preferredKey)) return preferredKey;
    const first = sessions.find((s) => s.status === "connected") ?? sessions[0];
    return first?.whatsapp_session_key ?? "";
  }, [sessions, preferredKey]);

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
