"use client";

import { useState } from "react";
import { Loader2, Pencil, Plus, ScrollText, Trash2, X } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/page-header";
import { SessionSelect } from "@/components/session-select";
import { useSessions } from "@/components/session-provider";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { usePoll } from "@/hooks/use-poll";
import { api, errMsg, fmtDate } from "@/lib/api";
import { WEBHOOK_EVENTS, type Webhook, type WebhookLog } from "@/lib/types";
import { cn } from "@/lib/utils";

type HeaderRow = { k: string; v: string };

function WebhookDialog({
  open,
  onOpenChange,
  editing,
  sessionKey,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  editing: Webhook | null;
  sessionKey: string;
  onSaved: () => void;
}) {
  const [url, setUrl] = useState("");
  const [events, setEvents] = useState<string[]>([]);
  const [headers, setHeaders] = useState<HeaderRow[]>([]);
  const [busy, setBusy] = useState(false);
  const [lastId, setLastId] = useState<string | null | undefined>(undefined);

  // Reinicia o formulário sempre que o diálogo abre para outro registro.
  const currentId = open ? (editing?.id ?? null) : undefined;
  if (currentId !== lastId) {
    setLastId(currentId);
    if (open) {
      setUrl(editing?.url ?? "");
      setEvents(editing?.events ?? ["message.received"]);
      setHeaders(Object.entries(editing?.headers ?? {}).map(([k, v]) => ({ k, v })));
    }
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (events.length === 0) {
      toast.error("Selecione ao menos um evento.");
      return;
    }
    const hdrs = Object.fromEntries(headers.filter((h) => h.k.trim()).map((h) => [h.k.trim(), h.v]));
    const body: Record<string, unknown> = { url: url.trim(), events };
    if (Object.keys(hdrs).length) body.headers = hdrs;

    setBusy(true);
    try {
      if (editing) await api(`webhooks/${editing.id}`, { method: "PUT", body, sessionKey });
      else await api("webhooks", { method: "POST", body, sessionKey });
      toast.success(editing ? "Webhook atualizado" : "Webhook criado");
      onOpenChange(false);
      onSaved();
    } catch (err) {
      toast.error(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{editing ? "Editar webhook" : "Novo webhook"}</DialogTitle>
            <DialogDescription>Sessão: {sessionKey}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="wurl">URL de destino</Label>
            <Input
              id="wurl"
              type="url"
              placeholder="https://seu-sistema.com/webhook"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label>Eventos</Label>
            <div className="grid gap-2 sm:grid-cols-2">
              {WEBHOOK_EVENTS.map((ev) => (
                <label key={ev} className="flex cursor-pointer items-center gap-2 rounded-md border border-border px-3 py-2 text-sm">
                  <Checkbox
                    checked={events.includes(ev)}
                    onCheckedChange={(c) => setEvents((p) => (c ? [...p, ev] : p.filter((x) => x !== ev)))}
                  />
                  <span className="font-mono text-xs">{ev}</span>
                </label>
              ))}
            </div>
          </div>
          <div className="space-y-2">
            <Label>Headers personalizados (opcional)</Label>
            {headers.map((h, i) => (
              <div key={i} className="flex gap-2">
                <Input
                  placeholder="Nome"
                  value={h.k}
                  onChange={(e) => setHeaders((p) => p.map((x, j) => (j === i ? { ...x, k: e.target.value } : x)))}
                />
                <Input
                  placeholder="Valor"
                  value={h.v}
                  onChange={(e) => setHeaders((p) => p.map((x, j) => (j === i ? { ...x, v: e.target.value } : x)))}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Remover header"
                  onClick={() => setHeaders((p) => p.filter((_, j) => j !== i))}
                >
                  <X />
                </Button>
              </div>
            ))}
            <Button type="button" variant="outline" size="sm" onClick={() => setHeaders((p) => [...p, { k: "", v: "" }])}>
              <Plus /> Adicionar header
            </Button>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy}>
              {busy && <Loader2 className="animate-spin" />} Salvar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function LogsDialog({
  webhook,
  sessionKey,
  onClose,
}: {
  webhook: Webhook | null;
  sessionKey: string;
  onClose: () => void;
}) {
  const { data, loading } = usePoll(
    () => (webhook ? api<WebhookLog[]>(`webhooks/${webhook.id}/logs?limit=50`, { sessionKey }) : Promise.resolve([])),
    0,
    [webhook?.id],
  );
  const [openId, setOpenId] = useState<string | null>(null);

  return (
    <Dialog open={!!webhook} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Logs de entrega</DialogTitle>
          <DialogDescription className="break-all">{webhook?.url}</DialogDescription>
        </DialogHeader>
        {loading ? (
          <Skeleton className="h-24" />
        ) : !data?.length ? (
          <p className="py-6 text-center text-sm text-muted-foreground">Nenhuma entrega registrada.</p>
        ) : (
          <ul className="space-y-2">
            {data.map((l) => {
              const ok = !!l.http_status && l.http_status >= 200 && l.http_status < 300;
              return (
                <li key={l.id} className="rounded-lg border border-border">
                  <button
                    className="flex w-full flex-wrap items-center gap-2 px-3 py-2 text-left text-sm"
                    onClick={() => setOpenId(openId === l.id ? null : l.id)}
                  >
                    <Badge
                      variant="outline"
                      className={cn(
                        ok ? "border-success/40 bg-success/15 text-success" : "border-destructive/40 bg-destructive/15 text-destructive",
                      )}
                    >
                      {l.http_status ?? "ERRO"}
                    </Badge>
                    <span className="font-mono text-xs">{l.event_type}</span>
                    <span className="ml-auto text-xs text-muted-foreground">
                      {l.execution_time_ms != null ? `${l.execution_time_ms} ms · ` : ""}
                      {fmtDate(l.triggered_at)}
                    </span>
                  </button>
                  {openId === l.id && (
                    <div className="space-y-2 border-t border-border px-3 py-2">
                      {l.error_message && <p className="text-xs text-destructive">{l.error_message}</p>}
                      <p className="text-xs text-muted-foreground">Payload</p>
                      <pre className="max-h-48 overflow-auto rounded bg-muted p-2 text-xs">{pretty(l.payload)}</pre>
                      {l.response_body && (
                        <>
                          <p className="text-xs text-muted-foreground">Resposta</p>
                          <pre className="max-h-32 overflow-auto rounded bg-muted p-2 text-xs">{l.response_body}</pre>
                        </>
                      )}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </DialogContent>
    </Dialog>
  );
}

function pretty(s: string) {
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

export default function WebhooksPage() {
  const { selectedKey } = useSessions();
  const hooks = usePoll(
    () => (selectedKey ? api<Webhook[]>("webhooks", { sessionKey: selectedKey }) : Promise.resolve([] as Webhook[])),
    0,
    [selectedKey],
  );
  const [dialog, setDialog] = useState<{ open: boolean; editing: Webhook | null }>({ open: false, editing: null });
  const [logsFor, setLogsFor] = useState<Webhook | null>(null);
  const [toDelete, setToDelete] = useState<Webhook | null>(null);

  async function remove() {
    if (!toDelete) return;
    try {
      await api(`webhooks/${toDelete.id}`, { method: "DELETE", sessionKey: selectedKey });
      toast.success("Webhook removido");
      await hooks.reload();
    } catch (e) {
      toast.error(errMsg(e));
    } finally {
      setToDelete(null);
    }
  }

  return (
    <>
      <PageHeader
        title="Webhooks"
        description="Receba eventos do WhatsApp em tempo real no seu sistema."
        actions={
          <>
            <SessionSelect />
            <Button disabled={!selectedKey} onClick={() => setDialog({ open: true, editing: null })}>
              <Plus /> Novo webhook
            </Button>
          </>
        }
      />

      <Card>
        <CardContent className="space-y-3">
          {!selectedKey ? (
            <p className="py-8 text-center text-sm text-muted-foreground">Crie uma sessão para configurar webhooks.</p>
          ) : hooks.loading ? (
            <Skeleton className="h-24" />
          ) : hooks.error ? (
            <p className="py-8 text-center text-sm text-destructive">{hooks.error.message}</p>
          ) : !hooks.data?.length ? (
            <p className="py-8 text-center text-sm text-muted-foreground">Nenhum webhook nesta sessão.</p>
          ) : (
            hooks.data.map((w) => (
              <div key={w.id} className="rounded-lg border border-border p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="break-all font-mono text-sm">{w.url}</p>
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {w.events.map((e) => (
                        <Badge key={e} variant="secondary" className="font-mono text-[10px]">
                          {e}
                        </Badge>
                      ))}
                    </div>
                    <p className="mt-2 text-xs text-muted-foreground">
                      Último disparo: {fmtDate(w.last_triggered_at)} · Criado em {fmtDate(w.created_at)}
                    </p>
                  </div>
                  <div className="flex gap-1">
                    <Button variant="ghost" size="sm" onClick={() => setLogsFor(w)}>
                      <ScrollText /> Logs
                    </Button>
                    <Button variant="ghost" size="icon-sm" aria-label="Editar" onClick={() => setDialog({ open: true, editing: w })}>
                      <Pencil />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="Excluir"
                      className="text-destructive hover:text-destructive"
                      onClick={() => setToDelete(w)}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>

      <WebhookDialog
        open={dialog.open}
        onOpenChange={(o) => setDialog((d) => ({ ...d, open: o }))}
        editing={dialog.editing}
        sessionKey={selectedKey}
        onSaved={() => void hooks.reload()}
      />
      <LogsDialog webhook={logsFor} sessionKey={selectedKey} onClose={() => setLogsFor(null)} />

      <AlertDialog open={!!toDelete} onOpenChange={(o) => !o && setToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Excluir webhook?</AlertDialogTitle>
            <AlertDialogDescription className="break-all">{toDelete?.url}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancelar</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void remove()}>
              Excluir
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
