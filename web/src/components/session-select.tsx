"use client";

import { useSessions } from "@/components/session-provider";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export function SessionSelect({ className, onlyConnected }: { className?: string; onlyConnected?: boolean }) {
  const { sessions, selectedKey, setSelectedKey } = useSessions();
  const list = onlyConnected ? sessions.filter((s) => s.status === "connected") : sessions;

  if (!list.length) {
    return (
      <span className="text-sm text-muted-foreground">
        Nenhuma sessão {onlyConnected ? "conectada" : "cadastrada"}
      </span>
    );
  }
  return (
    <Select value={selectedKey} onValueChange={(v) => v && setSelectedKey(v)}>
      <SelectTrigger className={className ?? "w-64"}>
        <SelectValue placeholder="Selecione a sessão" />
      </SelectTrigger>
      <SelectContent>
        {list.map((s) => (
          <SelectItem key={s.id} value={s.whatsapp_session_key}>
            {s.whatsapp_session_key}
            {s.phone_number ? ` · ${s.phone_number}` : ""}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
