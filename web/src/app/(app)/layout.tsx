import { AppShell } from "@/components/app-shell";

/**
 * Área autenticada: o AppShell só renderiza as páginas depois de confirmar o
 * login (/api/auth/me), então não há "app shell" estático para validar a
 * navegação instantânea. Optamos por não validar este segmento.
 */
export const instant = false;

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
