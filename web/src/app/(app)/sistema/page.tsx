"use client";

import { useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { usePoll } from "@/hooks/use-poll";
import { api, apiPlain, errMsg, fmtDate } from "@/lib/api";
import type { Health } from "@/lib/types";

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border py-2.5 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  );
}

export default function SystemPage() {
  const { data: h, loading, error, reload } = usePoll(() => apiPlain<Health>("health"), 15_000);
  const [refreshing, setRefreshing] = useState(false);

  async function refreshVersion() {
    setRefreshing(true);
    try {
      const d = await api<{ version: string }>("version/refresh", { method: "POST" });
      toast.success("Versão do WhatsApp Web atualizada", { description: d?.version });
      await reload();
    } catch (e) {
      toast.error(errMsg(e));
    } finally {
      setRefreshing(false);
    }
  }

  return (
    <>
      <PageHeader title="Sistema" description="Saúde da API e versão do cliente WhatsApp Web." />

      {error && (
        <p className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          API indisponível: {error.message}
        </p>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Saúde</CardTitle>
            <CardDescription>Atualiza a cada 15 segundos.</CardDescription>
          </CardHeader>
          <CardContent>
            {loading ? (
              <Skeleton className="h-40" />
            ) : (
              <>
                <Row
                  label="Status"
                  value={
                    <Badge
                      variant="outline"
                      className={
                        h?.status === "healthy"
                          ? "border-success/40 bg-success/15 text-success"
                          : "border-destructive/40 bg-destructive/15 text-destructive"
                      }
                    >
                      {h?.status ?? "desconhecido"}
                    </Badge>
                  }
                />
                <Row label="Serviço" value={h?.service ?? "—"} />
                <Row label="Versão da API" value={h?.version ?? "—"} />
                <Row label="Uptime" value={h?.uptime?.split(".")[0] ?? "—"} />
                <Row label="Hora do servidor" value={fmtDate(h?.timestamp)} />
                {Object.entries(h?.checks ?? {}).map(([k, v]) => (
                  <Row key={k} label={k.replaceAll("_", " ")} value={<span className="font-mono">{v}</span>} />
                ))}
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Cliente WhatsApp Web</CardTitle>
            <CardDescription>
              A versão é buscada no WhatsApp Web ao iniciar, a cada hora e automaticamente ao receber o erro 405 (cliente desatualizado).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-lg border border-border bg-muted/40 p-4">
              <p className="text-xs text-muted-foreground">Versão em uso</p>
              <p className="brand-gradient-text mt-1 font-mono text-2xl font-semibold">{h?.whatsapp_version ?? "—"}</p>
            </div>
            <Button onClick={() => void refreshVersion()} disabled={refreshing}>
              {refreshing ? <Loader2 className="animate-spin" /> : <RefreshCw />} Forçar atualização agora
            </Button>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
