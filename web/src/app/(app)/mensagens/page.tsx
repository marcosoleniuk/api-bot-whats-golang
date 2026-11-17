"use client";

import { useRef, useState } from "react";
import { BarChart3, CalendarClock, FileImage, Loader2, Mic, Plus, Send, Type, Video, X } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/page-header";
import { SessionSelect } from "@/components/session-select";
import { useSessions } from "@/components/session-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { api, errMsg, onlyDigits } from "@/lib/api";

interface SendResult {
  message_id?: string;
  recipient?: string;
  type?: string;
}

function readAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(String(r.result).split(",")[1] ?? "");
    r.onerror = () => reject(r.error);
    r.readAsDataURL(file);
  });
}

/** Hook comum: envia para um endpoint usando a sessão selecionada. */
function useSender() {
  const { selectedKey } = useSessions();
  const [busy, setBusy] = useState(false);

  async function send(path: string, body: unknown, okMsg = "Mensagem enviada") {
    if (!selectedKey) {
      toast.error("Selecione uma sessão conectada.");
      return false;
    }
    setBusy(true);
    try {
      const r = await api<SendResult>(path, { method: "POST", body, sessionKey: selectedKey });
      toast.success(okMsg, { description: r?.message_id ? `ID: ${r.message_id}` : undefined });
      return true;
    } catch (e) {
      toast.error(errMsg(e));
      return false;
    } finally {
      setBusy(false);
    }
  }
  return { send, busy };
}

function NumberField({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <div className="space-y-2">
      <Label htmlFor="number">Número (com DDI e DDD)</Label>
      <Input
        id="number"
        inputMode="numeric"
        placeholder="5544999999999"
        value={value}
        onChange={(e) => onChange(onlyDigits(e.target.value))}
        required
      />
      <p className="text-xs text-muted-foreground">Somente dígitos, de 10 a 15 caracteres.</p>
    </div>
  );
}

function SubmitButton({ busy, children }: { busy: boolean; children: React.ReactNode }) {
  return (
    <Button type="submit" disabled={busy} className="brand-gradient-bg border-0 text-white hover:opacity-90">
      {busy ? <Loader2 className="animate-spin" /> : <Send />} {children}
    </Button>
  );
}

function TextForm() {
  const { send, busy } = useSender();
  const [number, setNumber] = useState("");
  const [text, setText] = useState("");
  return (
    <form
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        if (await send("messages/text", { number, text })) setText("");
      }}
    >
      <NumberField value={number} onChange={setNumber} />
      <div className="space-y-2">
        <Label htmlFor="text">Mensagem</Label>
        <Textarea id="text" rows={5} value={text} onChange={(e) => setText(e.target.value)} required />
      </div>
      <SubmitButton busy={busy}>Enviar texto</SubmitButton>
    </form>
  );
}

/** Formulário de mídia reutilizado para imagem/documento, áudio e vídeo. */
function MediaForm({ kind }: { kind: "media" | "audio" | "video" }) {
  const { send, busy } = useSender();
  const [number, setNumber] = useState("");
  const [caption, setCaption] = useState("");
  const [url, setUrl] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [ptt, setPtt] = useState(true);
  const [gif, setGif] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const accept = { media: "image/*,application/pdf,.doc,.docx,.xls,.xlsx", audio: "audio/*", video: "video/*" }[kind];

  return (
    <form
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        if (!file && !url.trim()) {
          toast.error("Informe uma URL ou escolha um arquivo.");
          return;
        }
        const body: Record<string, unknown> = { number };
        if (file) {
          body.media_base64 = await readAsBase64(file);
          if (file.type) body.mime_type = file.type;
        } else {
          body.media_url = url.trim();
        }
        if (kind !== "audio" && caption) body.caption = caption;
        if (kind === "audio") body.ptt = ptt;
        if (kind === "video") body.gif_playback = gif;
        if (await send(`messages/${kind}`, body)) {
          setFile(null);
          setUrl("");
          setCaption("");
          if (fileRef.current) fileRef.current.value = "";
        }
      }}
    >
      <NumberField value={number} onChange={setNumber} />
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor={`file-${kind}`}>Arquivo</Label>
          <Input
            id={`file-${kind}`}
            ref={fileRef}
            type="file"
            accept={accept}
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor={`url-${kind}`}>…ou URL pública</Label>
          <Input
            id={`url-${kind}`}
            type="url"
            placeholder="https://exemplo.com/arquivo"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            disabled={!!file}
          />
        </div>
      </div>
      {kind !== "audio" && (
        <div className="space-y-2">
          <Label htmlFor={`cap-${kind}`}>Legenda (opcional)</Label>
          <Textarea id={`cap-${kind}`} rows={3} value={caption} onChange={(e) => setCaption(e.target.value)} />
        </div>
      )}
      {kind === "audio" && (
        <div className="flex items-center gap-3">
          <Switch checked={ptt} onCheckedChange={setPtt} id="ptt" />
          <Label htmlFor="ptt">Enviar como mensagem de voz (PTT)</Label>
        </div>
      )}
      {kind === "video" && (
        <div className="flex items-center gap-3">
          <Switch checked={gif} onCheckedChange={setGif} id="gif" />
          <Label htmlFor="gif">Reproduzir como GIF</Label>
        </div>
      )}
      <SubmitButton busy={busy}>Enviar</SubmitButton>
    </form>
  );
}

function PollForm() {
  const { send, busy } = useSender();
  const [number, setNumber] = useState("");
  const [question, setQuestion] = useState("");
  const [options, setOptions] = useState(["", ""]);
  const [max, setMax] = useState(1);

  return (
    <form
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        const opts = options.map((o) => o.trim()).filter(Boolean);
        if (opts.length < 2) {
          toast.error("A enquete precisa de pelo menos 2 opções.");
          return;
        }
        await send("messages/poll", {
          number,
          question,
          options: opts,
          selectable_options_count: Math.min(max, opts.length),
        });
      }}
    >
      <NumberField value={number} onChange={setNumber} />
      <div className="space-y-2">
        <Label htmlFor="q">Pergunta</Label>
        <Input id="q" value={question} onChange={(e) => setQuestion(e.target.value)} required />
      </div>
      <div className="space-y-2">
        <Label>Opções</Label>
        {options.map((o, i) => (
          <div key={i} className="flex gap-2">
            <Input
              value={o}
              placeholder={`Opção ${i + 1}`}
              onChange={(e) => setOptions((p) => p.map((x, j) => (j === i ? e.target.value : x)))}
            />
            {options.length > 2 && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label="Remover opção"
                onClick={() => setOptions((p) => p.filter((_, j) => j !== i))}
              >
                <X />
              </Button>
            )}
          </div>
        ))}
        {options.length < 12 && (
          <Button type="button" variant="outline" size="sm" onClick={() => setOptions((p) => [...p, ""])}>
            <Plus /> Adicionar opção
          </Button>
        )}
      </div>
      <div className="w-40 space-y-2">
        <Label htmlFor="max">Máx. de respostas</Label>
        <Input id="max" type="number" min={1} max={12} value={max} onChange={(e) => setMax(Number(e.target.value) || 1)} />
      </div>
      <SubmitButton busy={busy}>Enviar enquete</SubmitButton>
    </form>
  );
}

function EventForm() {
  const { send, busy } = useSender();
  const [number, setNumber] = useState("");
  const [f, setF] = useState({ name: "", description: "", join_link: "", start_time: "", end_time: "", call_type: "" });
  const [guests, setGuests] = useState(false);
  const [reminder, setReminder] = useState(false);
  const [offset, setOffset] = useState(15);

  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setF((p) => ({ ...p, [k]: e.target.value }));
  const iso = (v: string) => (v.length === 16 ? `${v}:00` : v);

  return (
    <form
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        const body: Record<string, unknown> = {
          number,
          name: f.name,
          start_time: iso(f.start_time),
          extra_guests_allowed: guests,
          has_reminder: reminder,
        };
        if (f.description) body.description = f.description;
        if (f.join_link) body.join_link = f.join_link;
        if (f.end_time) body.end_time = iso(f.end_time);
        if (f.call_type) body.call_type = f.call_type;
        if (reminder) body.reminder_offset_sec = offset * 60;
        await send("messages/event", body, "Evento enviado");
      }}
    >
      <NumberField value={number} onChange={setNumber} />
      <div className="space-y-2">
        <Label htmlFor="ename">Nome do evento</Label>
        <Input id="ename" value={f.name} onChange={set("name")} required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="edesc">Descrição</Label>
        <Textarea id="edesc" rows={3} value={f.description} onChange={set("description")} />
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="estart">Início (futuro)</Label>
          <Input id="estart" type="datetime-local" value={f.start_time} onChange={set("start_time")} required />
        </div>
        <div className="space-y-2">
          <Label htmlFor="eend">Fim (opcional)</Label>
          <Input id="eend" type="datetime-local" value={f.end_time} onChange={set("end_time")} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="elink">Link de participação</Label>
          <Input id="elink" type="url" value={f.join_link} onChange={set("join_link")} placeholder="https://" />
        </div>
        <div className="space-y-2">
          <Label htmlFor="ecall">Tipo de chamada</Label>
          <select
            id="ecall"
            value={f.call_type}
            onChange={(e) => setF((p) => ({ ...p, call_type: e.target.value }))}
            className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm dark:bg-input/30"
          >
            <option value="">Nenhuma</option>
            <option value="voice">Voz</option>
            <option value="video">Vídeo</option>
          </select>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-6">
        <div className="flex items-center gap-3">
          <Switch checked={guests} onCheckedChange={setGuests} id="guests" />
          <Label htmlFor="guests">Permitir convidados extras</Label>
        </div>
        <div className="flex items-center gap-3">
          <Switch checked={reminder} onCheckedChange={setReminder} id="rem" />
          <Label htmlFor="rem">Lembrete</Label>
          {reminder && (
            <Input
              type="number"
              min={1}
              value={offset}
              onChange={(e) => setOffset(Number(e.target.value) || 1)}
              className="w-20"
              aria-label="Minutos antes"
            />
          )}
          {reminder && <span className="text-sm text-muted-foreground">min antes</span>}
        </div>
      </div>
      <SubmitButton busy={busy}>Enviar evento</SubmitButton>
    </form>
  );
}

export default function MessagesPage() {
  const { selected } = useSessions();
  const notConnected = selected && selected.status !== "connected";

  return (
    <>
      <PageHeader
        title="Enviar mensagens"
        description="Envie texto, mídia, enquetes e eventos pelas suas sessões."
        actions={<SessionSelect onlyConnected />}
      />
      {notConnected && (
        <p className="mb-4 rounded-lg border border-brand-orange/40 bg-brand-orange/10 px-4 py-3 text-sm text-brand-orange">
          A sessão selecionada não está conectada; o envio deve falhar.
        </p>
      )}
      <Card>
        <CardHeader>
          <CardTitle>Nova mensagem</CardTitle>
          <CardDescription>
            Enviando como <span className="font-mono text-foreground">{selected?.whatsapp_session_key ?? "—"}</span>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="text">
            <TabsList className="mb-4 h-auto w-full flex-wrap justify-start">
              <TabsTrigger value="text"><Type /> Texto</TabsTrigger>
              <TabsTrigger value="media"><FileImage /> Imagem / Arquivo</TabsTrigger>
              <TabsTrigger value="audio"><Mic /> Áudio</TabsTrigger>
              <TabsTrigger value="video"><Video /> Vídeo</TabsTrigger>
              <TabsTrigger value="poll"><BarChart3 /> Enquete</TabsTrigger>
              <TabsTrigger value="event"><CalendarClock /> Evento</TabsTrigger>
            </TabsList>
            <TabsContent value="text"><TextForm /></TabsContent>
            <TabsContent value="media"><MediaForm kind="media" /></TabsContent>
            <TabsContent value="audio"><MediaForm kind="audio" /></TabsContent>
            <TabsContent value="video"><MediaForm kind="video" /></TabsContent>
            <TabsContent value="poll"><PollForm /></TabsContent>
            <TabsContent value="event"><EventForm /></TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </>
  );
}
