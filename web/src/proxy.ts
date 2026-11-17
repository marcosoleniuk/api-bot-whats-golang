import { NextResponse, type NextRequest } from "next/server";

const TOKEN_COOKIE = "wb_token";
const TENANT_COOKIE = "wb_tenant";

export function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl;
  const authed = Boolean(req.cookies.get(TOKEN_COOKIE)?.value && req.cookies.get(TENANT_COOKIE)?.value);

  if (pathname === "/login") {
    if (authed) return NextResponse.redirect(new URL("/", req.url));
    return NextResponse.next();
  }

  if (!authed) {
    const url = new URL("/login", req.url);
    if (pathname !== "/") url.searchParams.set("next", pathname);
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  // Páginas apenas; /api/* trata a própria autenticação.
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|logo.png).*)"],
};
