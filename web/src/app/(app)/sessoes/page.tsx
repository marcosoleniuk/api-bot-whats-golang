"use client";

import { useEffect, useState } from "react";
import { Loader2, Plus, Power, QrCode, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/page-header";
import { useSessions } from "@/components/session-provider";
import { StatusBadge } from "@/components/status-badge";
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api, errMsg, fmtDate, toImgSrc } from "@/lib/api";
import type { WaSession } from "@/lib/types";

interface QrData {
  qr_code_base64?: string;
  status?: string;
  qr_code_required?: boolean;
}

/** Mostra o QR code e acompanha o pareamento até a sessão conectar. */
function QrDialog({ sessionKey, onClose }: { sessionKey: string | null; onClose: () => void }) {
  const { reload } = useSessions();
  const [qr, setQr] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!sessionKey) return;
    let stop = false;

    const tick = async () => {
      try {
        const d = await api<QrData>(`whatsapp/qrcode/${encodeURIComponent(sessionKey)}`);
        if (stop) return;
        if (d?.qr_code_required === false || d?.status === "connected") {
          setConnected(true);
          setQr(null);
          void reload();
          return;
        }
        if (d?.qr_code_base64) setQr(d.qr_code_base64);
        setError("");
      } catch (e) {
        if (!stop) setError(errMsg(e));
      }
      if (!stop) timer = setTimeout(tick, 3000);
    };
    let timer = setTimeout(tick, 0);
    return () => {
      stop = true;
      clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionKey]);

  return (
    <Dialog open={!!sessionKey} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Conectar “{sessionKey}”</DialogTitle>
          <DialogDescription>
            No celular: WhatsApp → Aparelhos conectados → Conectar um aparelho, e escaneie o código.
          </DialogDescription>
        </DialogHeader>
        <div className="flex min-h-64 items-center justify-center">
          {connected ? (
            <div className="text-center">
              <Badge className="border-success/40 bg-success/15 text-success" variant="outline">
                Sessão conectada
              </Badge>
              <p className="mt-3 text-sm text-muted-foreground">O WhatsApp foi pareado com sucesso.</p>
            </div>
          ) : qr ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={toImgSrc(qr)} alt="QR code do WhatsApp" className="size-60 rounded-lg bg-white p-2" />
          ) : error ? (
            <p className="text-center text-sm text-muted-foreground">{error}</p>
          ) : (
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Fechar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function NewSessionDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  onCreated: (key: string) => void;
}) {
  const [key, setKey] = useState("");
  const [nome, setNome] = useState("");
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await api("whatsapp/register", {
        method: "POST",
        body: { whatsappSessionKey: key.trim(), nomePessoa: nome.trim(), emailPessoa: email.trim() },
      });
      toast.success("Sessão criada. Escaneie o QR code.");
      onOpenChange(false);
      onCreated(key.trim());
      setKey("");
      setNome("");
      setEmail("");
    } catch (err) {
      toast.error(errMsg(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Nova sessão WhatsApp</DialogTitle>
            <DialogDescription>Cadastre um número e depois escaneie o QR code.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="skey">Chave da sessão</Label>
            <Input id="skey" value={key} onChange={(e) => setKey(e.target.value)} placeholder="ex.: atendimento" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="snome">Nome</Label>
            <Input id="snome" value={nome} onChange={(e) => setNome(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="semail">E-mail</Label>
            <Input id="semail" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy}>
              {busy && <Loader2 className="animate-spin" />}
              Criar e gerar QR
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function SessionsPage() {
  const { sessions, loading, reload } = useSessions();
  const [creating, setCreating] = useState(false);
  const [qrKey, setQrKey] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<WaSession | null>(null);
  const [busyKey, setBusyKey] = useState<string | null>(null);

  async function disconnect(s: WaSession) {
    setBusyKey(s.whatsapp_session_key);
    try {
      await api(`whatsapp/disconnect/${encodeURIComponent(s.whatsapp_session_key)}`, { method: "POST" });
      toast.success(`Sessão ${s.whatsapp_session_key} desconectada`);
      await reload();
    } catch (e) {
      toast.error(errMsg(e));
    } finally {
      setBusyKey(null);
    }
  }

  async function remove() {
    if (!toDelete) return;
    const s = toDelete;
    setBusyKey(s.whatsapp_session_key);
    try {
      await api(`whatsapp/sessions/${encodeURIComponent(s.whatsapp_session_key)}`, { method: "DELETE" });
      toast.success(`Sessão ${s.whatsapp_session_key} removida`);
      await reload();
    } catch (e) {
      toast.error(errMsg(e));
    } finally {
      setBusyKey(null);
      setToDelete(null);
    }
  }

  return (
    <>
      <PageHeader
        title="Sessões"
        description="Gerencie os números WhatsApp conectados à API."
        actions={
          <>
            <Button variant="outline" onClick={() => void reload()}>
              <RefreshCw /> Atualizar
            </Button>
            <Button onClick={() => setCreating(true)}>
              <Plus /> Nova sessão
            </Button>
          </>
        }
      />

      <Card>
        <CardContent className="px-0">
          {loading ? (
            <div className="space-y-2 p-4">
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </div>
          ) : sessions.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">Nenhuma sessão cadastrada.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Sessão</TableHead>
                  <TableHead>Responsável</TableHead>
                  <TableHead>Telefone</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Última conexão</TableHead>
                  <TableHead className="pr-4 text-right">Ações</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map((s) => {
                  const busy = busyKey === s.whatsapp_session_key;
                  return (
                    <TableRow key={s.id}>
                      <TableCell className="pl-4 font-medium">{s.whatsapp_session_key}</TableCell>
                      <TableCell>
                        <div>{s.nome_pessoa}</div>
                        <div className="text-xs text-muted-foreground">{s.email_pessoa}</div>
                      </TableCell>
                      <TableCell className="font-mono text-xs">{s.phone_number ?? "—"}</TableCell>
                      <TableCell>
                        <StatusBadge status={s.status} />
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">{fmtDate(s.last_connected_at)}</TableCell>
                      <TableCell className="pr-4">
                        <div className="flex justify-end gap-1">
                          {s.status !== "connected" && (
                            <Button variant="ghost" size="sm" onClick={() => setQrKey(s.whatsapp_session_key)}>
                              <QrCode /> QR
                            </Button>
                          )}
                          {s.status === "connected" && (
                            <Button variant="ghost" size="sm" disabled={busy} onClick={() => void disconnect(s)}>
                              {busy ? <Loader2 className="animate-spin" /> : <Power />} Desconectar
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            className="text-destructive hover:text-destructive"
                            aria-label={`Excluir ${s.whatsapp_session_key}`}
                            disabled={busy}
                            onClick={() => setToDelete(s)}
                          >
                            <Trash2 />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <NewSessionDialog
        open={creating}
        onOpenChange={setCreating}
        onCreated={(k) => {
          void reload();
          setQrKey(k);
        }}
      />
      <QrDialog key={qrKey ?? "closed"} sessionKey={qrKey} onClose={() => setQrKey(null)} />

      <AlertDialog open={!!toDelete} onOpenChange={(o) => !o && setToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Excluir sessão “{toDelete?.whatsapp_session_key}”?</AlertDialogTitle>
            <AlertDialogDescription>
              A sessão será desconectada e removida. Esta ação não pode ser desfeita; será preciso parear novamente com um
              novo QR code.
            </AlertDialogDescription>
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
