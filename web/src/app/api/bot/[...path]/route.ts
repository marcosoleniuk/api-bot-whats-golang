import { NextResponse, type NextRequest } from "next/server";
import { BOT_API_URL, TENANT_COOKIE, TOKEN_COOKIE } from "@/lib/server-config";

/**
 * Proxy autenticado para a WhatsApp Bot API.
 *
 *   /api/bot/whatsapp/sessions  ->  {BOT_API_URL}/api/v1/whatsapp/sessions
 *   /api/bot/health             ->  {BOT_API_URL}/health
 *
 * O token da API nunca chega ao navegador: fica em cookie httpOnly e é
 * injetado aqui. O header `x-session-key` (opcional) escolhe qual chave de
 * sessão vai em X-WhatsApp-Session-Key; sem ele, usa a chave do tenant.
 */
type Ctx = { params: Promise<{ path: string[] }> };

async function handle(req: NextRequest, ctx: Ctx) {
  const { path } = await ctx.params;

  if (path.some((p) => p === ".." || p === "." || p.includes("\\"))) {
    return NextResponse.json({ message: "Caminho inválido" }, { status: 400 });
  }

  const token = req.cookies.get(TOKEN_COOKIE)?.value;
  const tenant = req.cookies.get(TENANT_COOKIE)?.value;
  if (!token || !tenant) {
    return NextResponse.json({ message: "Não autenticado" }, { status: 401, headers: { "x-auth-source": "proxy" } });
  }

  const sessionKey = req.headers.get("x-session-key")?.trim() || tenant;

  const isHealth = path.length === 1 && path[0] === "health";
  const search = new URLSearchParams(req.nextUrl.search);
  for (const k of ["apitoken", "SESSIONKEY", "session_key", "X-WhatsApp-Session-Key"]) search.delete(k);
  const qs = search.toString();
  const target = `${BOT_API_URL}${isHealth ? "/health" : `/api/v1/${path.map(encodeURIComponent).join("/")}`}${qs ? `?${qs}` : ""}`;

  const headers = new Headers({
    Authorization: `Bearer ${token}`,
    "X-WhatsApp-Session-Key": sessionKey,
  });
  const contentType = req.headers.get("content-type");
  if (contentType) headers.set("content-type", contentType);

  const hasBody = req.method !== "GET" && req.method !== "HEAD";

  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method: req.method,
      headers,
      body: hasBody ? await req.arrayBuffer() : undefined,
      cache: "no-store",
      signal: AbortSignal.timeout(90_000),
    });
  } catch {
    return NextResponse.json(
      { status: "error", message: "API do bot indisponível", code: "UPSTREAM_UNAVAILABLE" },
      { status: 502 },
    );
  }

  const out = new Headers();
  for (const h of ["content-type", "content-disposition", "content-length"]) {
    const v = upstream.headers.get(h);
    if (v) out.set(h, v);
  }
  out.set("cache-control", "no-store");
  return new Response(upstream.body, { status: upstream.status, headers: out });
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
