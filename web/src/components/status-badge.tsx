import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const MAP: Record<string, { label: string; cls: string }> = {
  connected: { label: "Conectada", cls: "border-success/40 bg-success/15 text-success" },
  pending: { label: "Aguardando QR", cls: "border-brand-orange/40 bg-brand-orange/15 text-brand-orange" },
  disconnected: { label: "Desconectada", cls: "border-border bg-muted text-muted-foreground" },
  error: { label: "Erro", cls: "border-destructive/40 bg-destructive/15 text-destructive" },
};

export function StatusBadge({ status }: { status: string }) {
  const s = MAP[status] ?? { label: status, cls: "border-border bg-muted text-muted-foreground" };
  return (
    <Badge variant="outline" className={cn("gap-1.5", s.cls)}>
      <span className="size-1.5 rounded-full bg-current" />
      {s.label}
    </Badge>
  );
}
