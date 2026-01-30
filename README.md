# WhatsApp Bot API Multi-Tenant - MOleniuk

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![Version](https://img.shields.io/badge/version-2.0.0-blue)](https://github.com/marcosoleniuk/api-bot-whats-golang)

Uma API REST profissional para gerenciamento de múltiplas sessões WhatsApp construída em Go, com arquitetura multi-tenant, suporte a Docker e configuração via variáveis de ambiente.

## 🚀 Características

- ✅ **Multi-Tenant**: Gerencie múltiplas sessões WhatsApp simultaneamente
- ✅ **Arquitetura Profissional**: Estrutura em camadas (handlers, services, middleware, repository)
- ✅ **Configuração via Ambiente**: Todas as configurações através de variáveis de ambiente
- ✅ **Logging Estruturado**: Sistema de logs profissional com níveis e contexto
- ✅ **Middleware de Autenticação**: Proteção com API Token e Session Key
- ✅ **Validação de Dados**: Validação robusta de entrada com feedback claro
- ✅ **Health Check**: Endpoint de monitoramento com status de conexões
- ✅ **Graceful Shutdown**: Desligamento elegante do servidor
- ✅ **Docker Ready**: Dockerfile multi-stage otimizado com Alpine Linux
- ✅ **Suporte a Mídia**: Envio de imagens, vídeos, áudios e documentos (URL e Base64)
- ✅ **Gerenciamento de Sessões**: Criação, listagem, desconexão e exclusão de sessões
- ✅ **QR Code**: Geração e atualização automática de QR codes para autenticação
- ✅ **Banco de Dados**: Suporte a PostgreSQL e SQLite3
- ✅ **Persistência**: Sessões persistidas com reconexão automática
- ✅ **API RESTful**: Endpoints bem estruturados e documentados

## 📋 Pré-requisitos

- Go 1.25 ou superior
- SQLite3 ou PostgreSQL
- Docker e Docker Compose (opcional)

## 🔧 Instalação

### Usando Go

1. Clone o repositório:

```bash
git clone https://github.com/marcosoleniuk/api-bot-whats-golang.git
cd api-bot-whats-golang
```

2. Copie o arquivo de exemplo de variáveis de ambiente:

```bash
cp .env.example .env
```

3. Edite o arquivo `.env` e configure suas credenciais:

```env
# Autenticação (OBRIGATÓRIO)
API_TOKEN=sua-api-key-segura-aqui
SESSION_KEY=sua-session-key-segura-aqui

# Banco de Dados - Escolha uma das opções:

# Opção 1: SQLite (recomendado para desenvolvimento/teste)
DB_DRIVER=sqlite3
DB_DSN=file:whatsapp.db?_foreign_keys=on

# Opção 2: PostgreSQL (recomendado para produção)
# DB_DRIVER=postgres
# DB_DSN=postgres://usuario:senha@localhost:5432/whatsapp_bot?sslmode=disable
```

**💡 Dica:** Gere tokens seguros em: https://www.strongdm.com/tools/api-key-generator

4. Instale as dependências:

```bash
go mod download
```

5. Execute a aplicação:

```bash
go run cmd/api/main.go
```

### Usando Docker

1. Clone o repositório:

```bash
git clone https://github.com/marcosoleniuk/api-bot-whats-golang.git
cd api-bot-whats-golang
```

2. Copie e configure o `.env`:

```bash
cp .env.example .env
```

**⚠️ IMPORTANTE:** Edite o arquivo `.env` e configure pelo menos:

- `API_TOKEN` - Token de autenticação da API (obrigatório)
- `SESSION_KEY` - Chave de sessão (obrigatório)
- `DB_DRIVER` - Driver do banco (sqlite3 ou postgres)
- `DB_DSN` - String de conexão do banco

**💡 Dica:** Gere tokens seguros em: https://www.strongdm.com/tools/api-key-generator

3. Execute com Docker Compose:

```bash
docker-compose up -d
```

4. Veja os logs:

```bash
docker-compose logs -f
```

## 📱 Gerenciamento de Sessões Multi-Tenant

Este sistema permite gerenciar múltiplas sessões WhatsApp simultaneamente. Cada sessão representa uma conta WhatsApp conectada.

### Primeiro Uso

1. **Registrar uma nova sessão:**

```bash
curl -X POST http://localhost:8080/api/v1/whatsapp/register \
  -H "apitoken: seu-api-token" \
  -H "SESSIONKEY: sua-session-key" \
  -H "Content-Type: application/json" \
  -d '{
    "session_key": "cliente-empresa-001",
    "nome_pessoa": "João Silva",
    "email_pessoa": "joao@empresa.com"
  }'
```

2. **Obter o QR Code para autenticação:**

```bash
curl -X GET http://localhost:8080/api/v1/whatsapp/qrcode/cliente-empresa-001 \
  -H "apitoken: seu-api-token" \
  -H "SESSIONKEY: sua-session-key"
```

3. **Escanear o QR Code:**
   - Abra o WhatsApp no seu celular
   - Vá em **Configurações** > **Aparelhos Conectados** > **Conectar um Aparelho**
   - Escaneie o QR Code retornado pela API

4. **Verificar status da conexão:**

```bash
curl -X GET http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: seu-api-token" \
  -H "SESSIONKEY: sua-session-key"
```

### Gestão de Sessões

**Listar todas as sessões:**

```bash
curl -X GET http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: seu-api-token" \
  -H "SESSIONKEY: sua-session-key"
```

**Desconectar uma sessão (sem deletar dados):**

```bash
curl -X POST http://localhost:8080/api/v1/whatsapp/disconnect/cliente-empresa-001 \
  -H "apitoken: seu-api-token" \
  -H "SESSIONKEY: sua-session-key"
```

**Deletar uma sessão permanentemente:**

```bash
curl -X DELETE http://localhost:8080/api/v1/whatsapp/sessions/cliente-empresa-001 \
  -H "apitoken: seu-api-token" \
  -H "SESSIONKEY: sua-session-key"
```

## 🔌 API Endpoints

Todas as requisições requerem os seguintes headers de autenticação:

```
apitoken: seu-api-token
SESSIONKEY: sua-session-key
Content-Type: application/json
```

### Gerenciamento de Sessões

#### 1. Registrar Nova Sessão

```http
POST /api/v1/whatsapp/register
```

**Body:**

```json
{
  "session_key": "cliente-empresa-001",
  "nome_pessoa": "João Silva",
  "email_pessoa": "joao@empresa.com"
}
```

**Resposta:**

```json
{
  "status": "success",
  "message": "Sessão registrada com sucesso. Use o endpoint /qrcode para obter o QR code.",
  "data": {
    "session_key": "cliente-empresa-001",
    "status": "pending",
    "created_at": "2026-01-30T10:30:00Z"
  }
}
```

#### 2. Obter QR Code de Sessão

```http
GET /api/v1/whatsapp/qrcode/{sessionKey}
```

**Resposta:**

```json
{
  "status": "success",
  "data": {
    "qr_code": "data:image/png;base64,iVBORw0KGgoAAAANS...",
    "expires_at": "2026-01-30T10:32:00Z",
    "session_status": "pending"
  }
}
```

#### 3. Listar Todas as Sessões

```http
GET /api/v1/whatsapp/sessions
```

**Resposta:**

```json
{
  "status": "success",
  "data": {
    "sessions": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "session_key": "cliente-empresa-001",
        "nome_pessoa": "João Silva",
        "email_pessoa": "joao@empresa.com",
        "phone_number": "5511999999999",
        "status": "connected",
        "created_at": "2026-01-30T10:00:00Z",
        "last_connected_at": "2026-01-30T10:30:00Z"
      }
    ],
    "total": 1
  }
}
```

#### 4. Desconectar Sessão

```http
POST /api/v1/whatsapp/disconnect/{sessionKey}
```

**Resposta:**

```json
{
  "status": "success",
  "message": "Sessão desconectada com sucesso"
}
```

#### 5. Deletar Sessão

```http
DELETE /api/v1/whatsapp/sessions/{sessionKey}
```

**Resposta:**

```json
{
  "status": "success",
  "message": "Sessão deletada com sucesso"
}
```

### Envio de Mensagens

#### 1. Enviar Mensagem de Texto

```http
POST /api/v1/messages/text
```

**Body:**

```json
{
  "session_key": "cliente-empresa-001",
  "number": "5511999999999",
  "text": "Olá! Esta é uma mensagem de teste."
}
```

**Resposta:**

```json
{
  "status": "success",
  "message": "Mensagem enviada com sucesso",
  "data": {
    "recipient": "5511999999999",
    "type": "text",
    "sent_at": "2026-01-30T10:30:00Z"
  }
}
```

#### 2. Enviar Mensagem com Mídia

```http
POST /api/v1/messages/media
```

**Body (com URL):**

```json
{
  "session_key": "cliente-empresa-001",
  "number": "5511999999999",
  "caption": "Confira esta imagem!",
  "media_url": "https://example.com/image.jpg"
}
```

**Body (com Base64):**

```json
{
  "session_key": "cliente-empresa-001",
  "number": "5511999999999",
  "caption": "Documento importante",
  "media_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
  "mime_type": "image/png"
}
```

**Resposta:**

```json
{
  "status": "success",
  "message": "Mensagem com mídia enviada com sucesso",
  "data": {
    "recipient": "5511999999999",
    "type": "media",
    "sent_at": "2026-01-30T10:30:00Z"
  }
}
```

### Health Check

```http
GET /health
```

**Resposta:**

```json
{
  "status": "healthy",
  "service": "WhatsApp Bot API (Multi-Tenant)",
  "version": "2.0.0",
  "uptime": "2h30m15s",
  "timestamp": "2026-01-30T10:30:00Z",
  "checks": {
    "whatsapp": "2 sessions connected",
    "database": "ok"
  }
}
```

## ⚙️ Variáveis de Ambiente

Todas as configurações são feitas através de variáveis de ambiente:

### Servidor

| Variável                  | Descrição                        | Padrão            |
| ------------------------- | -------------------------------- | ----------------- |
| `SERVER_PORT`             | Porta do servidor HTTP           | `8080`            |
| `SERVER_READ_TIMEOUT`     | Timeout de leitura               | `15s`             |
| `SERVER_WRITE_TIMEOUT`    | Timeout de escrita               | `15s`             |
| `SERVER_IDLE_TIMEOUT`     | Timeout de idle                  | `60s`             |
| `SERVER_SHUTDOWN_TIMEOUT` | Timeout de shutdown              | `10s`             |
| `MAX_UPLOAD_SIZE`         | Tamanho máximo de upload (bytes) | `52428800` (50MB) |

### WhatsApp

| Variável                   | Descrição                       | Padrão            |
| -------------------------- | ------------------------------- | ----------------- |
| `WHATSAPP_SESSION_KEY`     | Chave da sessão WhatsApp padrão | `default-session` |
| `WHATSAPP_DEFAULT_COUNTRY` | Código do país padrão           | `55`              |
| `WHATSAPP_QR_GENERATE`     | Gerar QR Code no terminal       | `true`            |
| `WHATSAPP_RECONNECT_DELAY` | Delay para reconexão            | `5s`              |

### Autenticação

| Variável      | Descrição                    | Obrigatório |
| ------------- | ---------------------------- | ----------- |
| `API_TOKEN`   | Token de autenticação da API | ✅ Sim      |
| `SESSION_KEY` | Chave de sessão              | ✅ Sim      |

### Banco de Dados

| Variável    | Descrição         | Exemplo                                                                    |
| ----------- | ----------------- | -------------------------------------------------------------------------- |
| `DB_DRIVER` | Driver do banco   | `sqlite3` ou `postgres`                                                    |
| `DB_DSN`    | String de conexão | `file:whatsapp.db?_foreign_keys=on` ou `postgres://user:pass@host:port/db` |

## 🏗️ Estrutura do Projeto

```
boot-whatsapp-golang/
├── cmd/
│   └── api/
│       └── main.go                    # Ponto de entrada da aplicação
├── internal/
│   ├── config/
│   │   └── config.go                  # Configuração centralizada
│   ├── handlers/
│   │   ├── handlers.go                # HTTP handlers (compatibilidade)
│   │   ├── multitenant_handler.go     # Handlers multi-tenant
│   │   └── session_handler.go         # Handlers de gerenciamento de sessões
│   ├── middleware/
│   │   └── middleware.go              # Middleware (auth, logging, recovery, CORS)
│   ├── models/
│   │   └── models.go                  # Estruturas de dados
│   ├── repository/
│   │   └── session_repository.go      # Camada de acesso aos dados
│   └── services/
│       ├── whatsapp.go                # Serviço WhatsApp (compatibilidade)
│       └── whatsapp_multitenant.go    # Serviço WhatsApp multi-tenant
├── migrations/
│   ├── 001_create_whatsapp_sessions.sql  # Migração inicial
│   └── 002_add_device_jid.sql            # Adiciona campo device_jid
├── pkg/
│   ├── logger/
│   │   └── logger.go                  # Sistema de logging estruturado
│   └── validator/
│       └── validator.go               # Validações de dados
├── .env.example                       # Exemplo de configuração
├── .gitignore                         # Arquivos ignorados pelo Git
├── docker-compose.yml                 # Configuração Docker Compose
├── Dockerfile                         # Dockerfile multi-stage otimizado
├── go.mod                             # Dependências Go
├── go.sum                             # Checksums das dependências
├── LICENSE                            # Licença MIT
└── README.md                          # Documentação
```

## 🔒 Segurança

- ✅ Autenticação via API Token e Session Key em todos os endpoints
- ✅ Validação de entrada em todas as requisições
- ✅ Sanitização de números de telefone
- ✅ Limitação de tamanho de upload (50MB padrão)
- ✅ CORS configurável via middleware
- ✅ Timeouts configurados para prevenir ataques
- ✅ Logs de tentativas de acesso não autorizado
- ✅ Isolamento de sessões (multi-tenant)
- ✅ Armazenamento seguro de credenciais no banco

## 📊 Monitoramento e Health Check

A API possui um endpoint de health check completo:

```bash
curl http://localhost:8080/health
```

**Resposta detalhada:**

```json
{
  "status": "healthy",
  "service": "WhatsApp Bot API (Multi-Tenant)",
  "version": "2.0.0",
  "uptime": "2h30m15s",
  "timestamp": "2026-01-30T10:30:00Z",
  "checks": {
    "whatsapp": "2 sessions connected",
    "database": "ok"
  }
}
```

Este endpoint verifica:

- ✅ Status geral do serviço
- ✅ Número de sessões WhatsApp conectadas
- ✅ Conectividade com o banco de dados
- ✅ Tempo de uptime do servidor
- ✅ Versão atual da API

## 🐛 Tratamento de Erros

Todos os erros seguem um formato padronizado JSON:

```json
{
  "status": "error",
  "message": "Descrição legível do erro",
  "code": "ERROR_CODE",
  "details": {
    "field": "informação adicional sobre o erro"
  },
  "timestamp": "2026-01-30T10:30:00Z"
}
```

### Códigos de Erro

| Código                  | Descrição                              | Status HTTP |
| ----------------------- | -------------------------------------- | ----------- |
| `AUTH_INVALID`          | Token ou session key inválidos         | 401         |
| `INVALID_JSON`          | Corpo da requisição malformado         | 400         |
| `VALIDATION_ERROR`      | Dados de entrada inválidos             | 400         |
| `INVALID_PHONE`         | Formato de número de telefone inválido | 400         |
| `SESSION_NOT_FOUND`     | Sessão WhatsApp não encontrada         | 404         |
| `SESSION_NOT_CONNECTED` | Sessão não está conectada              | 400         |
| `SEND_FAILED`           | Falha ao enviar mensagem               | 500         |
| `MEDIA_DOWNLOAD_FAILED` | Falha ao baixar mídia                  | 500         |
| `INTERNAL_ERROR`        | Erro interno do servidor               | 500         |

## 🧪 Testando a API

### Teste Rápido com cURL

```bash
# 1. Health Check
curl http://localhost:8080/health

# 2. Registrar nova sessão
curl -X POST http://localhost:8080/api/v1/whatsapp/register \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: sua-chave" \
  -H "Content-Type: application/json" \
  -d '{
    "session_key": "teste-001",
    "nome_pessoa": "Teste User",
    "email_pessoa": "teste@example.com"
  }'

# 3. Obter QR Code
curl http://localhost:8080/api/v1/whatsapp/qrcode/teste-001 \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: sua-chave"

# 4. Listar sessões
curl http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: sua-chave"

# 5. Enviar mensagem de texto
curl -X POST http://localhost:8080/api/v1/messages/text \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: sua-chave" \
  -H "Content-Type: application/json" \
  -d '{
    "session_key": "teste-001",
    "number": "5511999999999",
    "text": "Olá! Mensagem de teste."
  }'

# 6. Enviar imagem via URL
curl -X POST http://localhost:8080/api/v1/messages/media \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: sua-chave" \
  -H "Content-Type: application/json" \
  -d '{
    "session_key": "teste-001",
    "number": "5511999999999",
    "caption": "Imagem de teste",
    "media_url": "https://picsum.photos/800/600"
  }'
```

### Variáveis de Ambiente para Testes

Crie um arquivo `.env` com suas credenciais para facilitar os testes:

```env
API_TOKEN=seu-token-gerado
SESSION_KEY=sua-chave-gerada
```

## 🔄 Atualização e Manutenção

### Atualizando a Aplicação

```bash
# Com Docker
docker-compose down
git pull origin main
docker-compose up -d --build

# Sem Docker
git pull origin main
go mod download
go build -o whatsapp-bot cmd/api/main.go
./whatsapp-bot
```

### Backup do Banco de Dados

#### SQLite

```bash
# Backup
cp whatsapp.db whatsapp.db.backup

# Restore
cp whatsapp.db.backup whatsapp.db
```

#### PostgreSQL

```bash
# Backup
pg_dump -h localhost -U usuario whatsapp_bot > backup.sql

# Restore
psql -h localhost -U usuario whatsapp_bot < backup.sql
```

### Limpeza de Sessões Antigas

```bash
# Conectar ao banco e deletar sessões desconectadas há mais de 30 dias
# SQLite
sqlite3 whatsapp.db "DELETE FROM whatsapp_sessions WHERE status='disconnected' AND updated_at < datetime('now', '-30 days');"

# PostgreSQL
psql -c "DELETE FROM whatsapp_sessions WHERE status='disconnected' AND updated_at < NOW() - INTERVAL '30 days';"
```

## 📝 Logs e Debugging

### Níveis de Log

O sistema utiliza os seguintes níveis de log:

- `DEBUG`: Informações detalhadas para debugging
- `INFO`: Informações gerais de operação
- `WARN`: Avisos que não impedem a operação
- `ERROR`: Erros que afetam funcionalidades
- `FATAL`: Erros críticos que param a aplicação

### Exemplo de Logs

```
2026/01/30 10:30:00 [API] [INFO] Iniciando WhatsApp Bot API Multi-Tenant...
2026/01/30 10:30:01 [API] [INFO] Configuração carregada com sucesso
2026/01/30 10:30:02 [API] [INFO] Conectado ao banco de dados com sucesso
2026/01/30 10:30:03 [WhatsApp] [INFO] Carregando sessões existentes do banco de dados...
2026/01/30 10:30:04 [WhatsApp] [INFO] Encontradas 2 sessões no banco de dados
2026/01/30 10:30:05 [WhatsApp] [INFO] Sessão cliente-001 conectada com sucesso
2026/01/30 10:30:06 [API] [INFO] Servidor API escutando na porta 8080
```

### Visualizando Logs em Tempo Real

```bash
# Docker
docker-compose logs -f

# Docker (apenas últimas 100 linhas)
docker-compose logs -f --tail=100

# Docker (específico do serviço)
docker logs -f whatsapp-bot-api-golang
```

## 🛠️ Tecnologias Utilizadas

- **Go 1.25+** - Linguagem de programação
- **whatsmeow** - Biblioteca WhatsApp Web API
- **gorilla/mux** - Roteador HTTP
- **SQLite3 / PostgreSQL** - Banco de dados
- **Docker** - Containerização
- **Alpine Linux** - Imagem base otimizada

### Principais Dependências

```go
github.com/gorilla/mux v1.8.1          // Router HTTP
github.com/joho/godotenv v1.5.1        // Carregamento de .env
go.mau.fi/whatsmeow v0.0.0-...         // WhatsApp Web API
github.com/mattn/go-sqlite3 v1.14.33   // Driver SQLite
github.com/lib/pq v1.11.1              // Driver PostgreSQL
github.com/google/uuid v1.6.0          // Geração de UUIDs
github.com/skip2/go-qrcode v0.0.0-...  // Geração de QR codes
```

## 🤝 Contribuindo

Contribuições são bem-vindas! Por favor:

1. Faça um fork do projeto
2. Crie uma branch para sua feature (`git checkout -b feature/MinhaFeature`)
3. Commit suas mudanças (`git commit -m 'Adiciona MinhaFeature'`)
4. Push para a branch (`git push origin feature/MinhaFeature`)
5. Abra um Pull Request

### Diretrizes de Contribuição

- Mantenha o código limpo e bem documentado
- Siga as convenções de código Go
- Adicione testes quando apropriado
- Atualize a documentação conforme necessário

## 📄 Licença

Este projeto está sob a licença MIT. Veja o arquivo [LICENSE](LICENSE) para mais detalhes.

## ❓ FAQ (Perguntas Frequentes)

### Como adicionar múltiplas sessões WhatsApp?

Use o endpoint `/api/v1/whatsapp/register` para cada nova sessão com um `session_key` único.

### A sessão precisa ser reautenticada toda vez?

Não. As sessões são persistidas no banco de dados e reconectam automaticamente.

### Posso usar em produção?

Sim! Recomendamos usar PostgreSQL e Docker para ambientes de produção.

### Como limitar o acesso por IP?

Configure um reverse proxy (nginx, traefik) com regras de IP whitelisting.

### É possível enviar mensagens para grupos?

Sim, use o JID do grupo no campo `number`. Exemplo: `123456789-1234567890@g.us`

### Como configurar PostgreSQL?

Edite o `.env`:

```env
DB_DRIVER=postgres
DB_DSN=postgres://user:password@localhost:5432/whatsapp_bot?sslmode=disable
```

Execute as migrações em `migrations/` no PostgreSQL antes de iniciar.

## 🐛 Troubleshooting

### Problema: QR Code não aparece

**Solução:**

- Verifique se `WHATSAPP_QR_GENERATE=true` está configurado
- Acesse o endpoint `/api/v1/whatsapp/qrcode/{sessionKey}` diretamente

### Problema: Sessão desconecta frequentemente

**Solução:**

- Verifique a conexão de internet
- Certifique-se de que o celular está conectado
- Aumente `WHATSAPP_RECONNECT_DELAY` no `.env`

### Problema: Erro de autenticação

**Solução:**

- Verifique se `API_TOKEN` e `SESSION_KEY` estão corretos
- Confirme os headers `apitoken` e `SESSIONKEY` na requisição

### Problema: Falha ao enviar mídia

**Solução:**

- Verifique se a URL da mídia é acessível publicamente
- Para Base64, verifique se o `mime_type` está correto
- Confirme se o arquivo não excede `MAX_UPLOAD_SIZE`

### Problema: Banco de dados bloqueado (SQLite)

**Solução:**

- Migre para PostgreSQL em produção
- Ou aumente o timeout de lock no SQLite

## 🆘 Suporte

Se você encontrar problemas:

1. **Verifique os logs:**

   ```bash
   docker-compose logs -f
   ```

2. **Teste o health check:**

   ```bash
   curl http://localhost:8080/health
   ```

## 📊 Status do Projeto

![GitHub last commit](https://img.shields.io/github/last-commit/marcosoleniuk/api-bot-whats-golang)
![GitHub issues](https://img.shields.io/github/issues/marcosoleniuk/api-bot-whats-golang)
![GitHub stars](https://img.shields.io/github/stars/marcosoleniuk/api-bot-whats-golang)

---

## 👨‍💻 Autor

**Marcos Oleniuk**

- 📧 Email: marcos@moleniuk.com
- 💼 GitHub: [@marcosoleniuk](https://github.com/marcosoleniuk)
- 💬 WhatsApp: [+55 44 98809-9508](https://wa.me/5544988099508)

---

<div align="center">

**⭐ Se este projeto foi útil, considere dar uma estrela!**

Desenvolvido usando Golang

</div>
