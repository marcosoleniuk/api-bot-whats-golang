import type { Envelope } from "./types";

export class ApiError extends Error {
  status: number;
  code?: string;
  details?: Record<string, string>;
  constructor(message: string, status: number, code?: string, details?: Record<string, string>) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

interface Options {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  /** Chave de sessão a enviar em X-WhatsApp-Session-Key (padrão: chave do tenant). */
  sessionKey?: string;
  signal?: AbortSignal;
}

/** Chama a Bot API via proxy autenticado (/api/bot/*). Retorna o envelope completo. */
export async function apiRaw<T = unknown>(path: string, opts: Options = {}): Promise<Envelope<T>> {
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers["content-type"] = "application/json";
  if (opts.sessionKey) headers["x-session-key"] = opts.sessionKey;

  const res = await fetch(`/api/bot/${path.replace(/^\/+/, "")}`, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
    cache: "no-store",
  });

  // 401 vindo do proxy (cookie ausente/expirado) -> volta para o login.
  if (res.status === 401 && res.headers.get("x-auth-source") === "proxy" && typeof window !== "undefined") {
    window.location.href = "/login";
    throw new ApiError("Sessão expirada", 401);
  }

  const text = await res.text();
  let json: Envelope<T> | undefined;
  try {
    json = text ? (JSON.parse(text) as Envelope<T>) : undefined;
  } catch {
    json = undefined;
  }

  if (!res.ok) {
    const detail = json?.details?.error;
    const base = json?.message ?? (text.trim() || `Erro ${res.status}`);
    throw new ApiError(detail ? `${base}: ${detail}` : base, res.status, json?.code, json?.details);
  }
  return json ?? ({ status: "success", message: "" } as Envelope<T>);
}

/** Atalho que retorna apenas `data`. */
export async function api<T = unknown>(path: string, opts: Options = {}): Promise<T> {
  const env = await apiRaw<T>(path, opts);
  return env.data as T;
}

export function errMsg(e: unknown): string {
  return e instanceof Error ? e.message : "Erro inesperado";
}

/** Base64 puro (PNG) ou data URL -> src de <img>. */
export function toImgSrc(b64: string): string {
  return b64.startsWith("data:") ? b64 : `data:image/png;base64,${b64}`;
}

export function fmtDate(v?: string | null): string {
  if (!v) return "—";
  const d = new Date(v.includes("T") ? v : v.replace(" ", "T"));
  return Number.isNaN(d.getTime()) ? v : d.toLocaleString("pt-BR");
}

export function onlyDigits(v: string): string {
  return v.replace(/\D/g, "");
}

/** Para endpoints que retornam o objeto direto, sem envelope (ex.: /health). */
export async function apiPlain<T = unknown>(path: string, opts: Options = {}): Promise<T> {
  return (await apiRaw<unknown>(path, opts)) as unknown as T;
}
