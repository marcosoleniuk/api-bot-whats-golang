import { NextResponse, type NextRequest } from "next/server";
import { TENANT_COOKIE, TOKEN_COOKIE } from "@/lib/server-config";

export async function GET(req: NextRequest) {
  const token = req.cookies.get(TOKEN_COOKIE)?.value;
  const tenant = req.cookies.get(TENANT_COOKIE)?.value;
  if (!token || !tenant) {
    return NextResponse.json({ authenticated: false }, { status: 401 });
  }
  return NextResponse.json({ authenticated: true, tenantKey: tenant });
}
