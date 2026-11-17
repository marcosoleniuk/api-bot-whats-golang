"use client";

import Image from "next/image";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [apiToken, setApiToken] = useState("");
  const [tenantKey, setTenantKey] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ apiToken, tenantKey }),
      });
      const data = (await res.json()) as { message?: string };
      if (!res.ok) {
        setError(data.message ?? "Falha ao entrar.");
        return;
      }
      const next = params.get("next");
      router.replace(next && next.startsWith("/") && !next.startsWith("//") ? next : "/");
      router.refresh();
    } catch {
      setError("Erro de rede. Tente novamente.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="w-full max-w-md">
      <div className="mb-6 flex flex-col items-center gap-3">
        <Image src="/logo.png" alt="Moleniuk — Sempre em evolução" width={160} height={160} priority className="rounded-full" />
      </div>
      <Card className="border-border/80">
        <CardHeader>
          <CardTitle className="text-xl">Painel de administração</CardTitle>
          <CardDescription>Acesse com o token da API e a chave do seu tenant.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="token">Token da API</Label>
              <Input
                id="token"
                type="password"
                autoComplete="current-password"
                value={apiToken}
                onChange={(e) => setApiToken(e.target.value)}
                placeholder="API_TOKEN configurado no servidor"
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="tenant">Chave do tenant</Label>
              <Input
                id="tenant"
                value={tenantKey}
                onChange={(e) => setTenantKey(e.target.value)}
                placeholder="Identificador usado em X-WhatsApp-Session-Key"
                autoComplete="off"
                required
              />
            </div>
            {error && (
              <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error}
              </p>
            )}
            <Button type="submit" className="brand-gradient-bg w-full border-0 text-white hover:opacity-90" disabled={loading}>
              {loading && <Loader2 className="animate-spin" />}
              Entrar
            </Button>
          </form>
        </CardContent>
      </Card>
      <p className="mt-4 text-center text-xs text-muted-foreground">Sempre em evolução</p>
    </div>
  );
}
