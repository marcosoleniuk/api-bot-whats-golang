# WhatsApp Bot API Multi Sessões - MOleniuk

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

API REST em Go para gerenciar múltiplas sessões do WhatsApp, enviar mensagens, persistir histórico, registrar webhooks e consumir eventos em tempo real via WebSocket.

## Implementações atuais

- Multi sessões com registro, QR Code, listagem, desconexão e remoção de sessão.
- Envio de mensagens de texto, mídia, áudio, vídeo, enquete e evento.
- Persistência de mensagens de entrada e saída no banco.
- Armazenamento local de mídia com limpeza automática por retenção.
- Histórico paginado de conversas, contatos e estatísticas por sessão.
- Download de anexos via endpoint próprio de mídia.
- Webhooks com logs de execução.
- WebSocket para eventos em tempo real.
- Compatibilidade com rotas legadas `/sendText`, `/sendMedia`, `/sendAudio`, `/sendVideo`, `/sendPoll` e `/sendEvent`.

## Collection do Postman

O projeto inclui a collection atualizada em [API BOT Whats Golang.postman_collection.json](API%20BOT%20Whats%20Golang.postman_collection.json).

Variáveis principais da collection:

- `baseUrl`: URL HTTP da API.
- `wsBaseUrl`: URL WebSocket da API.
- `apiToken`: valor do `API_TOKEN`.
- `tenantSessionKey`: valor usado nos endpoints de gerenciamento de sessões.
- `waSessionKey`: valor da `whatsappSessionKey` registrada.
- `sessionID`: UUID da sessão para endpoints administrativos de conversa.
- `messageID`: UUID da mensagem para baixar mídia.
- `webhookId`: UUID do webhook.

## Autenticação

### 1. Endpoints de gerenciamento de sessão

Use:

- Header `apitoken: <API_TOKEN>`
- Header `SESSIONKEY: <chave-do-tenant-ou-contexto>`

Esses endpoints são:

- `POST /api/v1/whatsapp/register`
- `GET /api/v1/whatsapp/qrcode/{sessionKey}`
- `GET /api/v1/whatsapp/sessions`
- `POST /api/v1/whatsapp/disconnect/{sessionKey}`
- `DELETE /api/v1/whatsapp/sessions/{sessionKey}`
- `GET /api/v1/conversations/{sessionID}*`
- `GET /api/v1/conversations/{sessionID}/contacts`
- `GET /api/v1/conversations/{sessionID}/stats`

### 2. Endpoints escopados por sessão WhatsApp

Use uma das opções abaixo:

- Header `apitoken: <API_TOKEN>`
- Ou `Authorization: Bearer <API_TOKEN>`

E também:

- Header `X-WhatsApp-Session-Key: <whatsappSessionKey>`

Esses endpoints são:

- Todos os `POST /api/v1/messages/*`
- `GET /api/v1/messages/history`
- `GET /api/v1/messages/contacts`
- `GET /api/v1/messages/stats`
- `GET /api/v1/messages/{messageID}/media/{filename}`
- `GET /api/v1/ws`
- Todos os `POST/GET/PUT/DELETE /api/v1/webhooks*`

### 3. Health check

`GET /health` não exige autenticação.

## Início rápido

### 1. Configurar o ambiente

```bash
cp .env.example .env
```

Exemplo mínimo:

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

### 2. Rodar com Go

```bash
go mod download
go run cmd/api/main.go
```

### 3. Rodar com Docker

```bash
docker-compose up -d
```

## Variáveis de ambiente

### Servidor

| Variável | Descrição | Padrão |
| --- | --- | --- |
| `SERVER_PORT` | Porta HTTP | `8080` |
| `SERVER_READ_TIMEOUT` | Timeout de leitura | `15s` |
| `SERVER_WRITE_TIMEOUT` | Timeout de escrita | `15s` |
| `SERVER_IDLE_TIMEOUT` | Timeout idle | `60s` |
| `SERVER_SHUTDOWN_TIMEOUT` | Timeout de desligamento | `10s` |
| `MAX_UPLOAD_SIZE` | Tamanho máximo de upload | `52428800` |

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
| `WHATSAPP_RECONNECT_DELAY` | Delay de reconexão | `5s` |

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

## Endpoints

### Health

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/health` | Status da aplicação e quantidade de sessões conectadas |

### Sessões

| Método | Rota | Descrição |
| --- | --- | --- |
| `POST` | `/api/v1/whatsapp/register` | Registra ou recria uma sessão e retorna QR code |
| `GET` | `/api/v1/whatsapp/qrcode/{sessionKey}` | Consulta QR code da sessão |
| `GET` | `/api/v1/whatsapp/sessions` | Lista sessões do contexto informado |
| `POST` | `/api/v1/whatsapp/disconnect/{sessionKey}` | Desconecta uma sessão |
| `DELETE` | `/api/v1/whatsapp/sessions/{sessionKey}` | Remove uma sessão |

### Mensagens

| Método | Rota | Descrição |
| --- | --- | --- |
| `POST` | `/api/v1/messages/text` | Envia mensagem de texto |
| `POST` | `/api/v1/messages/media` | Envia imagem, documento ou mídia genérica via URL/Base64 |
| `POST` | `/api/v1/messages/audio` | Envia áudio via URL/Base64 |
| `POST` | `/api/v1/messages/video` | Envia vídeo via URL/Base64 |
| `POST` | `/api/v1/messages/poll` | Envia enquete |
| `POST` | `/api/v1/messages/event` | Envia evento |

### Histórico e mídia

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/api/v1/messages/history` | Histórico da sessão autenticada |
| `GET` | `/api/v1/messages/contacts` | Contatos únicos da sessão autenticada |
| `GET` | `/api/v1/messages/stats` | Estatísticas da sessão autenticada |
| `GET` | `/api/v1/messages/{messageID}/media/{filename}` | Baixa ou faz stream do anexo |
| `GET` | `/api/v1/conversations/{sessionID}` | Histórico por `sessionID` |
| `GET` | `/api/v1/conversations/{sessionID}/contacts` | Contatos por `sessionID` |
| `GET` | `/api/v1/conversations/{sessionID}/stats` | Estatísticas por `sessionID` |

### Webhooks e tempo real

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/api/v1/ws` | WebSocket de eventos |
| `POST` | `/api/v1/webhooks` | Registra webhook |
| `GET` | `/api/v1/webhooks` | Lista webhooks |
| `GET` | `/api/v1/webhooks/{webhookId}` | Busca webhook |
| `PUT` | `/api/v1/webhooks/{webhookId}` | Atualiza webhook |
| `DELETE` | `/api/v1/webhooks/{webhookId}` | Remove webhook |
| `GET` | `/api/v1/webhooks/{webhookId}/logs` | Consulta logs de entrega |

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

```json
{
  "number": "5511999999999",
  "media_url": "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3",
  "mime_type": "audio/mpeg",
  "ptt": false
}
```

### Enviar vídeo

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
  "start_time": "2026-04-01T09:00:00",
  "end_time": "2026-04-01T10:00:00",
  "call_type": "video",
  "force_structured": true,
  "extra_guests_allowed": true,
  "is_schedule_call": true,
  "has_reminder": true,
  "reminder_offset_sec": 900
}
```

Regras importantes do endpoint de evento:

- `start_time` aceita RFC3339, `YYYY-MM-DD HH:MM[:SS]` e Unix timestamp em segundos ou milissegundos.
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

### Histórico da sessão autenticada

```bash
curl "http://localhost:8080/api/v1/messages/history?limit=50&offset=0&include_media=true&contact=5511999999999" \
  -H "apitoken: seu-token" \
  -H "X-WhatsApp-Session-Key: cliente-001"
```

Parâmetros disponíveis:

- `limit`
- `offset`
- `contact`
- `include_media=true`

### WebSocket

```text
ws://localhost:8080/api/v1/ws?apitoken=seu-token&session_key=cliente-001&whatsapp_session_key=cliente-001
```

## Formato de resposta

### Resposta padrão de sucesso

```json
{
  "status": "success",
  "message": "Mensagem enviada com sucesso",
  "data": {
    "message_id": "3EB0C767D8F2D2A2",
    "recipient": "5511999999999",
    "type": "text",
    "sent_at": "2026-03-03T12:00:00Z"
  },
  "timestamp": "2026-03-03T12:00:00Z"
}
```

### Resposta padrão de erro

```json
{
  "status": "error",
  "message": "Header X-WhatsApp-Session-Key é obrigatório",
  "code": "MISSING_SESSION_KEY",
  "timestamp": "2026-03-03T12:00:00Z"
}
```

### Resposta do histórico

`GET /api/v1/messages/history` e `GET /api/v1/conversations/{sessionID}` retornam:

```json
{
  "messages": [],
  "pagination": {
    "total": 0,
    "limit": 50,
    "offset": 0
  }
}
```

Quando a mensagem possui anexo, a API pode retornar:

- `media_download_url`
- `media_url`
- `mime_type`
- `media_base64` quando `include_media=true`

## Eventos emitidos

### Webhooks

Eventos aceitos no cadastro:

- `message.sent`
- `message.received`
- `message.read`
- `*`

Exemplo de payload:

```json
{
  "event": "message.received",
  "session_key": "cliente-001",
  "tenant_id": "cliente-001",
  "timestamp": "2026-03-03T12:00:00Z",
  "data": {
    "message_id": "ABCD1234",
    "from": "5511999999999@s.whatsapp.net",
    "type": "text",
    "text": "Olá",
    "timestamp": "2026-03-03T12:00:00Z"
  }
}
```

### WebSocket

Eventos publicados em tempo real:

- `ws.connected`
- `session.connected`
- `session.disconnected`
- `session.logged_out`
- `message.sent`
- `message.received`
- `message.receipt`

## Persistência de mídia e histórico

- Mensagens de entrada e saída são salvas na tabela `messages`.
- Mídias podem ser baixadas do WhatsApp e armazenadas localmente em `MEDIA_STORAGE_PATH`.
- Quando uma mídia está salva localmente, o sistema passa a referenciá-la em formato `local://...`.
- O endpoint `GET /api/v1/messages/{messageID}/media/{filename}` faz streaming da mídia a partir do arquivo local ou da URL original.
- O job de limpeza remove arquivos antigos com base em `MEDIA_RETENTION_DAYS` e `MEDIA_CLEANUP_INTERVAL`.
- Enquetes recebidas têm metadados persistidos em `poll_metadata` para resolução posterior de votos.

## Health check

Exemplo de resposta real:

```json
{
  "status": "healthy",
  "service": "WhatsApp Bot API (Multi Sessões)",
  "version": "2.0.0",
  "uptime": "1m12.345s",
  "timestamp": "2026-03-03T12:00:00Z",
  "checks": {
    "api": "ok",
    "total_sessions": "1",
    "connected_sessions": "1"
  }
}
```
