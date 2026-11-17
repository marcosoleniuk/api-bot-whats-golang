import "server-only";

/** URL base da WhatsApp Bot API (Go). Configure em .env.local */
export const BOT_API_URL = (process.env.BOT_API_URL ?? "http://localhost:8080").replace(/\/+$/, "");

export const TOKEN_COOKIE = "wb_token";
export const TENANT_COOKIE = "wb_tenant";

export const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: "lax" as const,
  secure: process.env.NODE_ENV === "production" && process.env.COOKIE_SECURE !== "false",
  path: "/",
  maxAge: 60 * 60 * 24 * 7,
};
