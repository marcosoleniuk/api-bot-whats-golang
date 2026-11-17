# WhatsApp Bot API Multi Sessões — Moleniuk

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

API REST em Go para gerenciar múltiplas sessões do WhatsApp, enviar mensagens, persistir histórico, registrar webhooks e consumir eventos em tempo real via WebSocket. Acompanha um **painel de administração web** (Next.js) na pasta [`web/`](web/).

## Sumário

- [Recursos](#recursos)
- [Estrutura do projeto](#estrutura-do-projeto)
- [Início rápido](#início-rápido)
- [Variáveis de ambiente](#variáveis-de-ambiente)
- [Autenticação](#autenticação)
- [Endpoints](#endpoints)
- [Payloads e exemplos](#payloads-e-exemplos)
- [Formato de resposta](#formato-de-resposta)
- [Webhooks e WebSocket](#webhooks-e-websocket)
- [Atualização automática da versão do WhatsApp Web](#atualização-automática-da-versão-do-whatsapp-web)
- [Monitor de conexão](#monitor-de-conexão)
- [Persistência de mídia e histórico](#persistência-de-mídia-e-histórico)
- [Painel de administração (web)](#painel-de-administração-web)
- [Collection do Postman](#collection-do-postman)
- [Solução de problemas](#solução-de-problemas)

## Recursos

- Multi sessões: registro, QR Code, listagem, desconexão e remoção.
- Envio de texto, mídia (imagem/documento), áudio (incluindo PTT), vídeo (incluindo GIF), enquete e evento.
- Persistência de mensagens de entrada e saída no banco (SQLite ou PostgreSQL).
- Armazenamento local de mídia com limpeza automática por retenção.
- Histórico paginado de conversas, contatos e estatísticas por sessão.
- Download de anexos por endpoint próprio.
- Webhooks com logs de entrega e cabeçalhos personalizados.
- WebSocket para eventos em tempo real.
- **Versão do cliente WhatsApp Web atualizada automaticamente** (na inicialização, a cada hora e ao receber o erro 405).
- **Monitor de conexão** que reconecta sessões pareadas que caírem.
- Rotas legadas `/sendText`, `/sendMedia`, `/sendAudio`, `/sendVideo`, `/sendPoll` e `/sendEvent`.
- Painel web de administração (Next.js + shadcn/ui).

## Estrutura do projeto

```text
cmd/api/            ponto de entrada (main.go) e roteamento
internal/config/    leitura de variáveis de ambiente
internal/handlers/  handlers HTTP (sessões, mensagens, conversas, webhooks, WebSocket)
internal/services/  regras de negócio (WhatsApp multi tenant, webhooks, mensagens, tempo real)
internal/repository acesso ao banco
internal/middleware autenticação, CORS, logs e recovery
internal/models/    modelos e DTOs
dependencies/       cópia local (vendorizada) da biblioteca whatsmeow
migrations/         scripts SQL
pkg/                logger, mídia e validação
web/                painel de administração (Next.js)
```

> A biblioteca `whatsmeow` é usada a partir da pasta `dependencies/` (`replace go.mau.fi/whatsmeow => ./dependencies` no `go.mod`).

## Início rápido

### 1. Configurar o ambiente

```bash
cp .env.example .env
```

Exemplo mínimo (SQLite, para desenvolvimento):

```env
SERVER_PORT=8080
API_TOKEN=seu-token
SESSION_KEY=chave-obrigatoria-para-boot
DB_DRIVER=sqlite3
DB_DSN=file:whatsapp.db?_foreign_keys=on
MEDIA_STORAGE_PATH=storage/media
MEDIA_RETENTION_DAYS=7
MEDIA_CLEANUP_INTERVAL=24h
```

Para produção, prefira PostgreSQL:

```env
DB_DRIVER=postgres
DB_DSN=postgres://usuario:senha@localhost:5432/whatsapp_bot?sslmode=disable
```

### 2. Rodar com Go

```bash
go mod download
go run cmd/api/main.go
```

### 3. Rodar com Docker

```bash
docker-compose up -d
```

O `docker-compose.yml` define a porta interna com a variável `SERVER_PORT`; confira que ela coincide com a porta publicada e com o healthcheck (`8080` por padrão no `Dockerfile`).

### 4. Verificar

```bash
curl http://localhost:8080/health
```

### 5. Criar a primeira sessão

1. `POST /api/v1/whatsapp/register` (ver [exemplo](#registrar-sessão)).
2. Escaneie o QR Code (`qr_code_base64` na resposta, ou `GET /api/v1/whatsapp/qrcode/{sessionKey}`) em **WhatsApp → Aparelhos conectados → Conectar um aparelho**.
3. Confirme em `GET /api/v1/whatsapp/sessions` que o `status` é `connected`.

## Variáveis de ambiente

### Servidor

| Variável | Descrição | Padrão |
| --- | --- | --- |
| `SERVER_PORT` | Porta HTTP | `8080` |
| `SERVER_READ_TIMEOUT` | Timeout de leitura | `15s` |
| `SERVER_WRITE_TIMEOUT` | Timeout de escrita | `15s` |
| `SERVER_IDLE_TIMEOUT` | Timeout idle | `60s` |
| `SERVER_SHUTDOWN_TIMEOUT` | Timeout de desligamento | `10s` |
| `MAX_UPLOAD_SIZE` | Tamanho máximo de upload (bytes) | `52428800` |

### Mídia

| Variável | Descrição | Padrão |
| --- | --- | --- |
| `MEDIA_STORAGE_PATH` | Pasta onde anexos são salvos localmente | `storage/media` |
| `MEDIA_RETENTION_DAYS` | Dias de retenção dos arquivos locais | `7` |
| `MEDIA_CLEANUP_INTERVAL` | Intervalo do job de limpeza | `24h` |

### WhatsApp

| Variável | Descrição | Padrão |
| --- | --- | --- |
| `WHATSAPP_SESSION_KEY` | Chave padrão | `default-session` |
| `WHATSAPP_DEFAULT_COUNTRY` | Código de país padrão | `55` |
| `WHATSAPP_QR_GENERATE` | Habilita QR no terminal | `true` |
| `WHATSAPP_RECONNECT_DELAY` | Intervalo do monitor de conexão (valores abaixo de `10s` viram `30s`) | `5s` |

### Autenticação

| Variável | Descrição | Obrigatória |
| --- | --- | --- |
| `API_TOKEN` | Token global da API | Sim |
| `SESSION_KEY` | Valor obrigatório para inicialização da aplicação | Sim |

### Banco

| Variável | Descrição | Exemplo |
| --- | --- | --- |
| `DB_DRIVER` | Driver do banco | `sqlite3` ou `postgres` |
| `DB_DSN` | String de conexão | `file:whatsapp.db?_foreign_keys=on` |

As migrations ficam em [`migrations/`](migrations/) (`001` a `007`).

## Autenticação

Todas as rotas `/api/v1/*` exigem **token** e **chave** em cada requisição.

**Token da API** — qualquer uma das formas:

- Header `apitoken: <API_TOKEN>`
- Header `Authorization: Bearer <API_TOKEN>`
- Query string `?apitoken=<API_TOKEN>` (útil para WebSocket)

**Chave** — qualquer uma das formas:

- Header `X-WhatsApp-Session-Key: <chave>`
- Header `SESSIONKEY: <chave>`
- Query string `?X-WhatsApp-Session-Key=`, `?SESSIONKEY=` ou `?session_key=`

O significado da chave depende do endpoint:

| Grupo | Endpoints | A chave é… |
| --- | --- | --- |
| Gerenciamento de sessões | `/whatsapp/*` | a chave do **tenant** (agrupa as sessões; use `SESSIONKEY`) |
| Escopo de sessão | `POST /messages/*`, `GET /messages/history\|contacts\|stats`, `/webhooks*`, `/ws` | a `whatsappSessionKey` da **sessão** (use `X-WhatsApp-Session-Key`) |
| Administrativo por ID | `/conversations/{sessionID}*` | a chave do tenant; a sessão vem do `sessionID` na rota |

Resposta `401` com `AUTH_INVALID` (token errado) ou `SESSION_KEY_REQUIRED` (chave ausente).

`GET /health` **não** exige autenticação.

> **Atenção:** a API não valida a chave do tenant contra uma lista; quem tem o `API_TOKEN` pode usar qualquer chave. Proteja o token.

## Endpoints

### Health

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/health` | Status, uptime, versão da API, versão do WhatsApp Web em uso e contagem de sessões |

### Sessões

| Método | Rota | Descrição |
| --- | --- | --- |
| `POST` | `/api/v1/whatsapp/register` | Registra (ou recria) uma sessão e retorna o QR code |
| `GET` | `/api/v1/whatsapp/qrcode/{sessionKey}` | Consulta o QR code da sessão (ou informa que já está conectada) |
| `GET` | `/api/v1/whatsapp/sessions` | Lista as sessões do tenant |
| `POST` | `/api/v1/whatsapp/disconnect/{sessionKey}` | Desconecta uma sessão |
| `DELETE` | `/api/v1/whatsapp/sessions/{sessionKey}` | Remove uma sessão |

Status possíveis de uma sessão: `pending` (aguardando QR), `connected`, `disconnected`, `error`.

### Mensagens

| Método | Rota | Descrição |
| --- | --- | --- |
| `POST` | `/api/v1/messages/text` | Envia texto |
| `POST` | `/api/v1/messages/media` | Envia imagem, documento ou mídia genérica via URL/Base64 |
| `POST` | `/api/v1/messages/audio` | Envia áudio via URL/Base64 |
| `POST` | `/api/v1/messages/video` | Envia vídeo via URL/Base64 |
| `POST` | `/api/v1/messages/poll` | Envia enquete |
| `POST` | `/api/v1/messages/event` | Envia evento |

### Histórico e mídia

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/api/v1/messages/history` | Histórico da sessão (header `X-WhatsApp-Session-Key`) |
| `GET` | `/api/v1/messages/contacts` | Contatos únicos da sessão |
| `GET` | `/api/v1/messages/stats` | Estatísticas da sessão |
| `GET` | `/api/v1/messages/{messageID}/media/{filename}` | Baixa ou faz stream do anexo |
| `GET` | `/api/v1/conversations/{sessionID}` | Histórico por `sessionID` |
| `GET` | `/api/v1/conversations/{sessionID}/contacts` | Contatos por `sessionID` |
| `GET` | `/api/v1/conversations/{sessionID}/stats` | Estatísticas por `sessionID` |

### Webhooks e tempo real

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/api/v1/ws` | WebSocket de eventos |
| `POST` | `/api/v1/webhooks` | Registra webhook |
| `GET` | `/api/v1/webhooks` | Lista webhooks da sessão |
| `GET` | `/api/v1/webhooks/{webhookId}` | Busca webhook |
| `PUT` | `/api/v1/webhooks/{webhookId}` | Atualiza webhook (`url`, `events`, `headers`) |
| `DELETE` | `/api/v1/webhooks/{webhookId}` | Remove webhook |
| `GET` | `/api/v1/webhooks/{webhookId}/logs?limit=50` | Logs de entrega |

### Versão do WhatsApp Web

| Método | Rota | Descrição |
| --- | --- | --- |
| `POST` | `/api/v1/version/refresh` | Força a busca da versão mais recente do WhatsApp Web |

### Aliases legados

| Método | Rota | Equivalente |
| --- | --- | --- |
| `POST` | `/api/v1/sendText` | `/api/v1/messages/text` |
| `POST` | `/api/v1/sendMedia` | `/api/v1/messages/media` |
| `POST` | `/api/v1/sendAudio` | `/api/v1/messages/audio` |
| `POST` | `/api/v1/sendVideo` | `/api/v1/messages/video` |
| `POST` | `/api/v1/sendPoll` | `/api/v1/messages/poll` |
| `POST` | `/api/v1/sendEvent` | `/api/v1/messages/event` |

## Payloads e exemplos

> Os corpos JSON são validados de forma estrita: **campos desconhecidos são rejeitados** (`INVALID_JSON`). O campo `number` deve conter apenas dígitos, com DDI e DDD (10 a 15 dígitos), por exemplo `5511999999999`.

### Registrar sessão

```bash
curl -X POST http://localhost:8080/api/v1/whatsapp/register \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: tenant-demo" \
  -H "Content-Type: application/json" \
  -d '{
    "whatsappSessionKey": "cliente-001",
    "nomePessoa": "João Silva",
    "emailPessoa": "joao@empresa.com"
  }'
```

Resposta (`201`):

```json
{
  "status": "success",
  "message": "Sessão registrada com sucesso. Escaneie o QR code para conectar.",
  "data": {
    "id": "c4c1efdf-5e86-438f-a595-6f2c1be7d463",
    "whatsapp_session_key": "cliente-001",
    "qr_code_base64": "iVBORw0KGgo...",
    "status": "pending",
    "expires_at": "2026-11-17T16:17:00Z"
  }
}
```

`qr_code_base64` é um PNG em Base64 puro (sem prefixo `data:`).

### Listar sessões

```bash
curl http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: tenant-demo"
```

```json
{
  "status": "success",
  "message": "Sessões listadas com sucesso",
  "data": {
    "total": 1,
    "sessions": [
      {
        "id": "c4c1efdf-5e86-438f-a595-6f2c1be7d463",
        "whatsapp_session_key": "cliente-001",
        "nome_pessoa": "João Silva",
        "email_pessoa": "joao@empresa.com",
        "phone_number": "5511999999999",
        "status": "connected",
        "created_at": "2026-11-17 16:16:00",
        "last_connected_at": "2026-11-17 16:29:44"
      }
    ]
  }
}
```

### Enviar texto

```bash
curl -X POST http://localhost:8080/api/v1/messages/text \
  -H "apitoken: seu-token" \
  -H "X-WhatsApp-Session-Key: cliente-001" \
  -H "Content-Type: application/json" \
  -d '{
    "number": "5511999999999",
    "text": "Olá"
  }'
```

### Enviar mídia

Imagem, documento ou mídia genérica. Informe `media_url` **ou** `media_base64`.

Via URL:

```json
{
  "number": "5511999999999",
  "caption": "Arquivo de teste",
  "media_url": "https://example.com/arquivo.pdf"
}
```

Via Base64:

```json
{
  "number": "5511999999999",
  "caption": "Imagem em base64",
  "media_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
  "mime_type": "image/png"
}
```

### Enviar áudio

`ptt: true` envia como mensagem de voz.

```json
{
  "number": "5511999999999",
  "media_url": "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3",
  "mime_type": "audio/mpeg",
  "ptt": false
}
```

### Enviar vídeo

`gif_playback: true` reproduz como GIF.

```json
{
  "number": "5511999999999",
  "caption": "Vídeo de teste",
  "media_url": "https://samplelib.com/lib/preview/mp4/sample-5s.mp4",
  "mime_type": "video/mp4",
  "gif_playback": false
}
```

### Enviar enquete

```json
{
  "number": "5511999999999",
  "question": "Qual opção você prefere?",
  "options": ["Opção A", "Opção B", "Opção C"],
  "selectable_options_count": 1
}
```

### Enviar evento

```json
{
  "number": "5511999999999",
  "name": "Reunião semanal",
  "description": "Alinhamento de sprint",
  "join_link": "https://call.whatsapp.com/video/SEU_TOKEN_REAL_AQUI",
  "start_time": "2026-12-01T09:00:00",
  "end_time": "2026-12-01T10:00:00",
  "call_type": "video",
  "force_structured": true,
  "extra_guests_allowed": true,
  "is_schedule_call": true,
  "has_reminder": true,
  "reminder_offset_sec": 900
}
```

Regras do endpoint de evento:

- `start_time` aceita RFC3339, `YYYY-MM-DDTHH:MM[:SS]`, `YYYY-MM-DD HH:MM[:SS]` e Unix timestamp (segundos ou milissegundos). Datas sem fuso usam o fuso do servidor.
- `start_time` precisa estar no futuro.
- `end_time` é opcional, mas não pode ser menor que `start_time`.
- `call_type` aceita apenas `voice` ou `video`.
- Se `has_reminder=true` e `reminder_offset_sec<=0`, a API assume `900`.

### Registrar webhook

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "apitoken: seu-token" \
  -H "X-WhatsApp-Session-Key: cliente-001" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://seu-servidor.com/webhook",
    "events": ["message.sent", "message.received", "message.read"],
    "headers": {
      "Authorization": "Bearer token-custom"
    }
  }'
```

### Histórico da sessão

```bash
curl "http://localhost:8080/api/v1/messages/history?limit=50&offset=0&include_media=true&contact=5511999999999" \
  -H "apitoken: seu-token" \
  -H "X-WhatsApp-Session-Key: cliente-001"
```

Parâmetros: `limit` (padrão 50), `offset` (padrão 0), `contact` (filtra por número) e `include_media=true` (inclui `media_base64`).

### Forçar atualização da versão do WhatsApp Web

```bash
curl -X POST http://localhost:8080/api/v1/version/refresh \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: tenant-demo"
```

```json
{ "status": "success", "message": "Versão do WhatsApp Web atualizada", "data": { "version": "2.3000.1054981628" } }
```

## Formato de resposta

### Sucesso

```json
{
  "status": "success",
  "message": "Mensagem enviada com sucesso",
  "data": {
    "message_id": "3EB0C767D8F2D2A2",
    "recipient": "5511999999999",
    "type": "text",
    "sent_at": "2026-11-17T17:39:49Z"
  },
  "timestamp": "2026-11-17T17:39:49Z"
}
```

### Erro

```json
{
  "status": "error",
  "message": "Header X-WhatsApp-Session-Key é obrigatório",
  "code": "MISSING_SESSION_KEY",
  "timestamp": "2026-11-17T17:39:49Z"
}
```

Códigos comuns: `AUTH_INVALID`, `SESSION_KEY_REQUIRED`, `MISSING_SESSION_KEY`, `INVALID_JSON`, `VALIDATION_ERROR`, `INVALID_PHONE`, `REGISTRATION_FAILED`, `QRCODE_NOT_FOUND`, `VERSION_REFRESH_FAILED`.

### Histórico

`GET /api/v1/messages/history` e `GET /api/v1/conversations/{sessionID}` retornam o objeto direto (sem o envelope `status/data`):

```json
{
  "messages": [],
  "pagination": { "total": 0, "limit": 50, "offset": 0 }
}
```

Mensagens com anexo podem trazer `media_download_url`, `media_url`, `mime_type` e `media_base64` (quando `include_media=true`).

### Health

`GET /health` também retorna o objeto direto:

```json
{
  "status": "healthy",
  "service": "WhatsApp Bot API (Multi Sessões)",
  "version": "2.0.0",
  "whatsapp_version": "2.3000.1054981628",
  "uptime": "1m12.345s",
  "timestamp": "2026-11-17T12:00:00Z",
  "checks": {
    "api": "ok",
    "total_sessions": "1",
    "connected_sessions": "1"
  }
}
```

## Webhooks e WebSocket

### Webhooks

Eventos aceitos no cadastro (`events`):

| Evento | Quando dispara |
| --- | --- |
| `message.sent` | Após enviar uma mensagem pela API |
| `message.received` | Ao receber uma mensagem |
| `message.read` | Quando uma mensagem enviada é lida |
| `*` | Todos os acima |

Payload entregue:

```json
{
  "event": "message.received",
  "session_key": "cliente-001",
  "tenant_id": "cliente-001",
  "timestamp": "2026-11-17T12:00:00Z",
  "data": {
    "message_id": "ABCD1234",
    "chat": "5511999999999@s.whatsapp.net",
    "from": "5511999999999@s.whatsapp.net",
    "type": "text",
    "text": "Olá",
    "timestamp": "2026-11-17T12:00:00Z"
  }
}
```

Cada entrega é registrada e consultável em `GET /api/v1/webhooks/{webhookId}/logs` (status HTTP, tempo de execução, corpo da resposta e erro).

### WebSocket

```text
ws://localhost:8080/api/v1/ws?apitoken=seu-token&session_key=cliente-001&whatsapp_session_key=cliente-001
```

Parâmetros de escopo: `session_key` (ou o header de chave), `whatsapp_session_key` e `scopes` (lista separada por vírgulas). Eventos publicados:

| Evento | Descrição |
| --- | --- |
| `ws.connected` | Confirmação da conexão |
| `session.connected` | Sessão conectou ao WhatsApp |
| `session.disconnected` | Sessão desconectou |
| `session.logged_out` | Sessão foi desconectada pelo celular |
| `session.client_outdated` | O WhatsApp recusou a versão do cliente (erro 405); a API tenta se corrigir sozinha |
| `message.sent` | Mensagem enviada |
| `message.received` | Mensagem recebida |
| `message.receipt` | Recibo de entrega/leitura |

## Atualização automática da versão do WhatsApp Web

O WhatsApp recusa clientes com versão antiga (erro `405 Client outdated`). A API mantém a versão sempre atual:

1. **Na inicialização:** busca a versão em `web.whatsapp.com` (3 tentativas).
2. **A cada 1 hora:** atualização periódica em segundo plano.
3. **Ao receber o erro 405:** busca a versão mais recente (3 tentativas) e reconecta a sessão afetada (no máximo uma vez por minuto).
4. **Manual:** `POST /api/v1/version/refresh` ou o botão em **Sistema** no painel web.

A versão em uso aparece em `GET /health` (`whatsapp_version`). Se o log mostrar *"Versão do WhatsApp Web não mudou … pode exigir atualização da biblioteca whatsmeow"*, a versão publicada não basta e é preciso atualizar a biblioteca em `dependencies/`.

## Monitor de conexão

A cada `WHATSAPP_RECONNECT_DELAY` (valores abaixo de 10 s viram 30 s) a API verifica as sessões pareadas e reconecta as que estiverem desconectadas. Sessões que nunca foram pareadas (sem QR escaneado) não são reconectadas: gere um novo QR code.

## Persistência de mídia e histórico

- Mensagens de entrada e saída são salvas na tabela `messages`.
- Mídias podem ser baixadas do WhatsApp e armazenadas em `MEDIA_STORAGE_PATH`; a referência passa a ser `local://...`.
- `GET /api/v1/messages/{messageID}/media/{filename}` faz streaming a partir do arquivo local ou da URL original.
- O job de limpeza remove arquivos antigos conforme `MEDIA_RETENTION_DAYS` e `MEDIA_CLEANUP_INTERVAL`.
- Enquetes recebidas têm metadados persistidos em `poll_metadata` para resolver votos depois.

## Painel de administração (web)

A pasta [`web/`](web/) contém um painel em **Next.js 16 + Tailwind 4 + shadcn/ui**, com tema escuro nas cores da logo.

```bash
cd web
cp .env.example .env.local   # BOT_API_URL=http://localhost:8080
pnpm install
pnpm dev                     # http://localhost:3000
```

Telas: Dashboard, Sessões (QR code com acompanhamento), Enviar mensagens (texto, mídia, áudio, vídeo, enquete, evento), Conversas, Webhooks (com logs) e Sistema (saúde e versão do WhatsApp Web).

O login usa o `API_TOKEN` e a chave do tenant. As credenciais ficam em cookies `httpOnly` e as chamadas passam pelo proxy `/api/bot/*`, então o token nunca chega ao navegador. Detalhes em [`web/README.md`](web/README.md).

## Collection do Postman

A collection está em [API BOT Whats Golang.postman_collection.json](API%20BOT%20Whats%20Golang.postman_collection.json) (formato v2.1). Importe-a no Postman e ajuste as variáveis.

| Variável | Descrição |
| --- | --- |
| `baseUrl` | URL HTTP da API |
| `wsBaseUrl` | URL WebSocket da API |
| `apiToken` | Valor do `API_TOKEN` |
| `tenantSessionKey` | Chave do tenant (gerenciamento de sessões) |
| `waSessionKey` | `whatsappSessionKey` da sessão |
| `phoneNumber` | Número de destino (só dígitos, com DDI) |
| `sessionID` | UUID da sessão (rotas `/conversations`) |
| `messageID` / `messageFilename` | UUID e nome do arquivo de uma mensagem com mídia |
| `webhookId` / `webhookUrl` | UUID e URL de destino do webhook |
| `eventStart` / `eventEnd` | Datas do evento (precisam estar no futuro) |
| `wsScopes` | Escopos extras do WebSocket |

A collection **salva automaticamente** `sessionID`, `webhookId` e `messageID` quando você registra/lista sessões, cria webhooks e envia mensagens, e traz testes básicos de status e formato em cada requisição.

## Solução de problemas

| Sintoma | Causa provável / solução |
| --- | --- |
| `Client outdated (405)` nos logs | A API busca a versão sozinha; se persistir, use `POST /api/v1/version/refresh` e veja o log de versão. Se a versão não mudar, atualize a biblioteca em `dependencies/`. |
| `sessão não pode ser reconectada: phone_number ausente` | Sessão nunca pareada. Registre de novo e escaneie o QR code. |
| `timeout aguardando QR code` ao registrar | O WhatsApp recusou a conexão (verifique o erro 405 acima) ou houve falha de rede. |
| `401 AUTH_INVALID` | `API_TOKEN` incorreto. |
| `401 SESSION_KEY_REQUIRED` | Falta `SESSIONKEY` / `X-WhatsApp-Session-Key`. |
| `INVALID_JSON` | Corpo com campo desconhecido ou formato inválido. |
| `INVALID_PHONE` | `number` deve ter só dígitos (10 a 15), com DDI. |
| Painel mostra dados diferentes da produção | `BOT_API_URL` do `web/.env.local` aponta para outra instância (ex.: local). |
| Mesma sessão em dois servidores | Um número pareado só pode estar ativo em uma instância por vez. |

## Licença

Apache 2.0 — veja [LICENSE](LICENSE).
