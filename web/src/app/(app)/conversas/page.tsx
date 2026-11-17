"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowDownLeft, ArrowUpRight, Contact, MessageCircle, RefreshCw, Users } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { SessionSelect } from "@/components/session-select";
import { useSessions } from "@/components/session-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { usePoll } from "@/hooks/use-poll";
import { api, apiPlain, fmtDate } from "@/lib/api";
import type { ChatMessage, Conversation, MessageStats } from "@/lib/types";
import { cn } from "@/lib/utils";

const PAGE = 100;

function MediaPreview({ m }: { m: ChatMessage }) {
  if (!m.media_download_url) return null;
  const src = m.media_download_url.replace(/^\/api\/v1\//, "/api/bot/");
  const type = m.message_type;
  if (type === "image" || type === "sticker")
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt="Imagem recebida" className="mb-1 max-h-64 rounded-md" loading="lazy" />;
  if (type === "video") return <video src={src} controls className="mb-1 max-h-64 rounded-md" />;
  if (type === "audio") return <audio src={src} controls className="mb-1 w-full" />;
  return (
    <a href={src} target="_blank" rel="noreferrer" className="mb-1 block text-sm underline">
      Abrir {type}
    </a>
  );
}

function Bubble({ m }: { m: ChatMessage }) {
  const out = m.direction === "outbound";
  return (
    <div className={cn("flex", out ? "justify-end" : "justify-start")}>
      <div
        className={cn(
          "max-w-[80%] rounded-2xl px-3 py-2 text-sm",
          out ? "rounded-br-sm bg-primary/90 text-primary-foreground" : "rounded-bl-sm bg-muted",
        )}
      >
        <MediaPreview m={m} />
        {m.content ? (
          <p className="whitespace-pre-wrap break-words">{m.content}</p>
        ) : !m.media_download_url ? (
          <p className="italic opacity-70">[{m.message_type}]</p>
        ) : null}
        <p className={cn("mt-1 text-[10px]", out ? "text-primary-foreground/70" : "text-muted-foreground")}>
          {fmtDate(m.created_at)} · {m.status}
        </p>
      </div>
    </div>
  );
}

export default function ConversationsPage() {
  const { selectedKey } = useSessions();
  const [contact, setContact] = useState("");
  const [filter, setFilter] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

  const stats = usePoll(
    () => (selectedKey ? api<MessageStats>("messages/stats", { sessionKey: selectedKey }) : Promise.resolve(undefined)),
    30_000,
    [selectedKey],
  );
  const contacts = usePoll(
    () =>
      selectedKey
        ? api<{ contacts: string[]; count: number }>("messages/contacts", { sessionKey: selectedKey })
        : Promise.resolve(undefined),
    30_000,
    [selectedKey],
  );
  const convo = usePoll(
    () =>
      selectedKey && contact
        ? apiPlain<Conversation>(
            `messages/history?contact=${encodeURIComponent(contact)}&limit=${PAGE}&offset=0&include_media=false`,
            { sessionKey: selectedKey },
          )
        : Promise.resolve(undefined),
    8_000,
    [selectedKey, contact],
  );

  useEffect(() => {
    setContact("");
  }, [selectedKey]);

  const list = useMemo(() => {
    const all = contacts.data?.contacts ?? [];
    const f = filter.trim();
    return f ? all.filter((c) => c.includes(f)) : all;
  }, [contacts.data, filter]);

  const messages = useMemo(
    () =>
      [...(convo.data?.messages ?? [])].sort(
        (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
      ),
    [convo.data],
  );

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end" });
  }, [messages.length, contact]);

  const s = stats.data;

  return (
    <>
      <PageHeader
        title="Conversas"
        description="Histórico de mensagens recebidas e enviadas por sessão."
        actions={
          <>
            <SessionSelect />
            <Button
              variant="outline"
              size="icon"
              aria-label="Atualizar"
              onClick={() => {
                void stats.reload();
                void contacts.reload();
                void convo.reload();
              }}
            >
              <RefreshCw />
            </Button>
          </>
        }
      />

      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          { label: "Total", value: s?.total_messages, icon: MessageCircle, cls: "text-brand-blue" },
          { label: "Recebidas", value: s?.inbound, icon: ArrowDownLeft, cls: "text-success" },
          { label: "Enviadas", value: s?.outbound, icon: ArrowUpRight, cls: "text-brand-orange" },
          { label: "Contatos", value: s?.contact_count, icon: Users, cls: "text-brand-blue" },
        ].map(({ label, value, icon: Icon, cls }) => (
          <Card key={label} size="sm">
            <CardContent className="flex items-center gap-3">
              <Icon className={cn("size-5", cls)} />
              <div>
                <p className="text-xs text-muted-foreground">{label}</p>
                <p className="text-xl font-semibold">{stats.loading ? "…" : (value ?? 0)}</p>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-[18rem_1fr]">
        <Card className="lg:h-[34rem]">
          <CardContent className="flex h-full flex-col gap-3">
            <Input placeholder="Filtrar contato…" value={filter} onChange={(e) => setFilter(e.target.value)} />
            <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
              {contacts.loading ? (
                <Skeleton className="h-24" />
              ) : list.length === 0 ? (
                <p className="py-6 text-center text-sm text-muted-foreground">Nenhum contato.</p>
              ) : (
                <ul className="space-y-1">
                  {list.map((c) => (
                    <li key={c}>
                      <button
                        onClick={() => setContact(c)}
                        className={cn(
                          "flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left font-mono text-xs transition-colors hover:bg-muted",
                          c === contact && "bg-muted shadow-[inset_2px_0_0_var(--brand-orange)]",
                        )}
                      >
                        <Contact className="size-4 shrink-0 text-brand-blue" />
                        <span className="truncate">{c}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </CardContent>
        </Card>

        <Card className="lg:h-[34rem]">
          <CardContent className="flex h-full flex-col">
            {!contact ? (
              <div className="flex flex-1 flex-col items-center justify-center gap-2 text-muted-foreground">
                <MessageCircle className="size-8" />
                <p className="text-sm">Selecione um contato para ver a conversa.</p>
              </div>
            ) : (
              <>
                <div className="mb-3 flex items-center justify-between border-b border-border pb-3">
                  <span className="font-mono text-sm">{contact}</span>
                  <span className="text-xs text-muted-foreground">
                    {convo.data?.pagination.total ?? 0} mensagens
                    {(convo.data?.pagination.total ?? 0) > PAGE ? ` (últimas ${PAGE})` : ""}
                  </span>
                </div>
                <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
                  {convo.loading && !convo.data ? (
                    <Skeleton className="h-20" />
                  ) : messages.length === 0 ? (
                    <p className="py-8 text-center text-sm text-muted-foreground">Sem mensagens.</p>
                  ) : (
                    messages.map((m) => <Bubble key={m.id} m={m} />)
                  )}
                  <div ref={bottomRef} />
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}
