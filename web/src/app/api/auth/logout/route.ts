import { NextResponse } from "next/server";
import { TENANT_COOKIE, TOKEN_COOKIE } from "@/lib/server-config";

export async function POST() {
  const res = NextResponse.json({ ok: true });
  res.cookies.delete(TOKEN_COOKIE);
  res.cookies.delete(TENANT_COOKIE);
  return res;
}
