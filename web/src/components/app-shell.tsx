"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import {
  Activity,
  KeyRound,
  LayoutDashboard,
  LogOut,
  Menu,
  MessageSquare,
  MessagesSquare,
  Send,
  Smartphone,
  Webhook,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { SessionProvider } from "@/components/session-provider";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/sessoes", label: "Sessões", icon: Smartphone },
  { href: "/mensagens", label: "Enviar mensagens", icon: Send },
  { href: "/conversas", label: "Conversas", icon: MessagesSquare },
  { href: "/webhooks", label: "Webhooks", icon: Webhook },
  { href: "/sistema", label: "Sistema", icon: Activity },
] as const;

function Brand() {
  return (
    <Link href="/" className="flex items-center gap-3 px-2">
      <Image src="/logo.png" alt="Moleniuk" width={44} height={44} className="rounded-full" priority />
      <div className="leading-tight">
        <div className="brand-gradient-text text-base font-bold tracking-wide">MOLENIUK</div>
        <div className="text-[11px] text-muted-foreground">WhatsApp Admin</div>
      </div>
    </Link>
  );
}

function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();
  return (
    <nav className="flex flex-col gap-1">
      {NAV.map(({ href, label, icon: Icon }) => {
        const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
        return (
          <Link
            key={href}
            href={href}
            onClick={onNavigate}
            className={cn(
              "group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
              active
                ? "bg-sidebar-accent text-foreground shadow-[inset_2px_0_0_var(--brand-orange)]"
                : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-foreground",
            )}
          >
            <Icon className={cn("size-4", active ? "text-brand-orange" : "text-brand-blue")} />
            {label}
          </Link>
        );
      })}
    </nav>
  );
}

function SidebarBody({ tenantKey, onNavigate }: { tenantKey: string; onNavigate?: () => void }) {
  const router = useRouter();

  async function logout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.replace("/login");
    router.refresh();
  }

  return (
    <div className="flex h-full flex-col gap-6 p-4">
      <Brand />
      <NavLinks onNavigate={onNavigate} />
      <div className="mt-auto space-y-3">
        <div className="rounded-lg border border-border bg-card/60 p-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <KeyRound className="size-3.5 text-brand-orange" /> Tenant
          </div>
          <div className="mt-1 truncate font-mono text-xs" title={tenantKey}>
            {tenantKey}
          </div>
        </div>
        <Button variant="outline" className="w-full justify-start gap-2" onClick={logout}>
          <LogOut /> Sair
        </Button>
      </div>
    </div>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const [tenantKey, setTenantKey] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    fetch("/api/auth/me", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((d: { tenantKey: string }) => setTenantKey(d.tenantKey))
      .catch(() => {
        // Recarregamento completo é intencional: limpa todo o estado do cliente.
        // eslint-disable-next-line @next/next/no-location-assign-relative-destination
        window.location.assign("/login");
      });
  }, []);

  if (!tenantKey) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Skeleton className="size-16 rounded-full" />
      </div>
    );
  }

  return (
    <SessionProvider tenantKey={tenantKey}>
      <div className="min-h-screen lg:grid lg:grid-cols-[17rem_1fr]">
        <aside className="sticky top-0 hidden h-screen border-r border-sidebar-border bg-sidebar lg:block">
          <SidebarBody tenantKey={tenantKey} />
        </aside>

        <div className="flex min-w-0 flex-col">
          <header className="sticky top-0 z-30 flex items-center gap-3 border-b border-border bg-background/80 px-4 py-3 backdrop-blur lg:hidden">
            <Button variant="ghost" size="icon" onClick={() => setOpen(true)} aria-label="Abrir menu">
              <Menu />
            </Button>
            <MessageSquare className="size-4 text-brand-blue" />
            <span className="brand-gradient-text font-bold tracking-wide">MOLENIUK</span>
          </header>
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetContent side="left" className="w-72 bg-sidebar p-0">
              <SheetTitle className="sr-only">Menu</SheetTitle>
              <SidebarBody tenantKey={tenantKey} onNavigate={() => setOpen(false)} />
            </SheetContent>
          </Sheet>

          <main className="mx-auto w-full max-w-6xl flex-1 p-4 md:p-8">{children}</main>
        </div>
      </div>
    </SessionProvider>
  );
}
