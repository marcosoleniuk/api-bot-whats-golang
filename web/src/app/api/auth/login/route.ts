import { NextResponse, type NextRequest } from "next/server";
import { BOT_API_URL, COOKIE_OPTIONS, TENANT_COOKIE, TOKEN_COOKIE } from "@/lib/server-config";

export async function POST(req: NextRequest) {
  let body: { apiToken?: string; tenantKey?: string };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ message: "Corpo inválido" }, { status: 400 });
  }

  const apiToken = body.apiToken?.trim();
  const tenantKey = body.tenantKey?.trim();
  if (!apiToken || !tenantKey) {
    return NextResponse.json({ message: "Informe o token da API e a chave do tenant." }, { status: 400 });
  }

  // Valida as credenciais listando as sessões do tenant.
  let upstream: Response;
  try {
    upstream = await fetch(`${BOT_API_URL}/api/v1/whatsapp/sessions`, {
      headers: {
        Authorization: `Bearer ${apiToken}`,
        "X-WhatsApp-Session-Key": tenantKey,
      },
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    return NextResponse.json(
      { message: `Não foi possível conectar à API em ${BOT_API_URL}. Verifique se ela está no ar.` },
      { status: 502 },
    );
  }

  if (upstream.status === 401) {
    return NextResponse.json({ message: "Token da API inválido." }, { status: 401 });
  }
  if (!upstream.ok) {
    return NextResponse.json({ message: `A API respondeu com erro (${upstream.status}).` }, { status: 502 });
  }

  const res = NextResponse.json({ ok: true });
  res.cookies.set(TOKEN_COOKIE, apiToken, COOKIE_OPTIONS);
  res.cookies.set(TENANT_COOKIE, tenantKey, COOKIE_OPTIONS);
  return res;
}
