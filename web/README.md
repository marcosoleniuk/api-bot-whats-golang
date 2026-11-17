# Moleniuk • WhatsApp Admin

Painel de administração da WhatsApp Bot API (Go). Next.js 16 · React 19 · Tailwind 4 · shadcn/ui.
Tema escuro baseado nas cores da logo (azul `#1E7BFF` e laranja `#FF9A1F`).

## Rodando

```bash
cp .env.example .env.local     # ajuste BOT_API_URL
pnpm install
pnpm dev                       # http://localhost:3000
```

Produção:

```bash
pnpm build && pnpm start
```

Variáveis (`.env.local`):

| Variável | Padrão | Descrição |
|---|---|---|
| `BOT_API_URL` | `http://localhost:8080` | URL da API Go (sem `/api/v1`) |
| `COOKIE_SECURE` | `true` em produção | Use `false` se servir o painel via HTTP puro |

## Como funciona

- **Login:** token da API (`API_TOKEN`) + chave do tenant (valor enviado em `X-WhatsApp-Session-Key`). As credenciais
  ficam em cookies `httpOnly`; o navegador nunca vê o token.
- **Proxy:** tudo passa por `/api/bot/*` → `{BOT_API_URL}/api/v1/*` (e `/api/bot/health` → `/health`), o que evita CORS.
- **Sessão selecionada:** as rotas de mensagens, conversas e webhooks usam a chave da sessão escolhida no seletor
  como `X-WhatsApp-Session-Key`; a listagem de sessões usa a chave do tenant.

## Telas

Dashboard · Sessões (criar, QR code com acompanhamento, desconectar, excluir) · Enviar mensagens (texto, imagem/arquivo,
áudio, vídeo, enquete, evento) · Conversas (contatos, histórico, mídia) · Webhooks (CRUD, eventos, headers, logs) ·
Sistema (saúde e versão do WhatsApp Web, com atualização forçada).

## Limitações conhecidas

- O WebSocket de tempo real (`/api/v1/ws`) não é usado: o painel atualiza por polling (conversas a cada 8 s, sessões a cada 15 s).
- A API trata o tenant como a própria chave informada no header; qualquer pessoa com o token da API pode acessar
  qualquer tenant. Proteja o painel e o token adequadamente.
