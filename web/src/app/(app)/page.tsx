"use client";

import Link from "next/link";
import { Activity, CheckCircle2, Cpu, Smartphone, Timer, TriangleAlert } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { useSessions } from "@/components/session-provider";
import { StatusBadge } from "@/components/status-badge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { usePoll } from "@/hooks/use-poll";
import { apiPlain, fmtDate } from "@/lib/api";
import type { Health } from "@/lib/types";

function Stat({
  icon: Icon,
  label,
  value,
  hint,
  tone = "blue",
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: React.ReactNode;
  hint?: string;
  tone?: "blue" | "orange" | "green";
}) {
  const toneCls = { blue: "text-brand-blue", orange: "text-brand-orange", green: "text-success" }[tone];
  return (
    <Card>
      <CardContent className="flex items-start gap-4 pt-1">
        <div className="rounded-lg border border-border bg-muted p-2.5">
          <Icon className={`size-5 ${toneCls}`} />
        </div>
        <div className="min-w-0">
          <p className="text-sm text-muted-foreground">{label}</p>
          <p className="truncate text-2xl font-semibold tracking-tight">{value}</p>
          {hint && <p className="mt-0.5 truncate text-xs text-muted-foreground">{hint}</p>}
        </div>
      </CardContent>
    </Card>
  );
}

export default function DashboardPage() {
  const { sessions, loading: loadingSessions } = useSessions();
  const { data: health, error, loading } = usePoll(() => apiPlain<Health>("health"), 15_000);
  const h = health;
  const connected = sessions.filter((s) => s.status === "connected").length;
  const pending = sessions.filter((s) => s.status === "pending").length;
  const offline = sessions.length - connected - pending;

  return (
    <>
      <PageHeader title="Dashboard" description="Visão geral da WhatsApp Bot API e das suas sessões." />

      {error && (
        <div className="mb-4 flex items-center gap-2 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <TriangleAlert className="size-4" /> API indisponível: {error.message}
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-24 rounded-xl" />)
        ) : (
          <>
            <Stat
              icon={Activity}
              label="Status da API"
              value={h?.status === "healthy" ? "Saudável" : (h?.status ?? "—")}
              hint={h?.service}
              tone="green"
            />
            <Stat icon={Timer} label="Uptime" value={h?.uptime ? h.uptime.split(".")[0] : "—"} hint={`v${h?.version ?? "?"}`} />
            <Stat
              icon={Cpu}
              label="Versão do WhatsApp Web"
              value={<span className="font-mono text-xl">{h?.whatsapp_version ?? "—"}</span>}
              hint="Atualizada automaticamente"
              tone="orange"
            />
            <Stat
              icon={CheckCircle2}
              label="Sessões conectadas"
              value={`${connected} / ${sessions.length}`}
              hint={`${pending} aguardando QR · ${offline} offline`}
              tone="green"
            />
          </>
        )}
      </div>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Sessões</CardTitle>
          <CardDescription>Números WhatsApp cadastrados neste tenant.</CardDescription>
          <CardAction>
            <Button variant="outline" size="sm" render={<Link href="/sessoes" />}>
              Gerenciar
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          {loadingSessions ? (
            <Skeleton className="h-24 w-full" />
          ) : sessions.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <Smartphone className="size-8 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">Nenhuma sessão cadastrada ainda.</p>
              <Button size="sm" render={<Link href="/sessoes" />}>
                Criar primeira sessão
              </Button>
            </div>
          ) : (
            <ul className="divide-y divide-border">
              {sessions.map((s) => (
                <li key={s.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                  <div className="min-w-0">
                    <p className="font-medium">{s.whatsapp_session_key}</p>
                    <p className="truncate text-xs text-muted-foreground">
                      {s.nome_pessoa} · {s.email_pessoa}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    {s.phone_number && <Badge variant="secondary" className="font-mono">{s.phone_number}</Badge>}
                    <span className="hidden text-xs text-muted-foreground sm:inline">
                      {s.last_connected_at ? `Conectou ${fmtDate(s.last_connected_at)}` : "Nunca conectou"}
                    </span>
                    <StatusBadge status={s.status} />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </>
  );
}
