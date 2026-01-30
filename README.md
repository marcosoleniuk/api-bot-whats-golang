# WhatsApp Bot API Multi Sessões - MOleniuk

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![Version](https://img.shields.io/badge/version-2.0.0-blue)](https://github.com/marcosoleniuk/api-bot-whats-golang)
[![Security](https://img.shields.io/badge/security-multi--tenant%20isolation-green)](SECURITY_UPDATE.md)

Uma API REST profissional para gerenciamento de múltiplas sessões WhatsApp construída em Go, com arquitetura Multi Sessões, suporte a Docker e configuração via variáveis de ambiente.

---

## 🔐 Importante - Versão 2.0 com Isolamento Multi Sessões

**A partir da versão 2.0, implementamos isolamento completo de dados por tenant!**

- 🔒 Cada `SESSION_KEY` funciona como namespace isolado
- ✅ Impossível acessar dados de outros tenants
- ⚠️ Header `SESSIONKEY` agora é **obrigatório** em todas as requisições
- 📖 Veja [SECURITY_UPDATE.md](SECURITY_UPDATE.md) para detalhes da migração

**Migrando da v1.x?** Execute a migração `003_add_tenant_id.sql` e consulte a documentação de atualização.

---

## 🚀 Características

- ✅ **Multi Sessões com Isolamento**: Gerencie múltiplas sessões WhatsApp com isolamento completo por tenant
- 🔒 **Segurança por Tenant**: Cada `SESSION_KEY` só acessa suas próprias sessões (isolamento total)
- ✅ **Arquitetura Profissional**: Estrutura em camadas (handlers, services, middleware, repository)
- ✅ **Configuração via Ambiente**: Todas as configurações através de variáveis de ambiente
- ✅ **Logging Estruturado**: Sistema de logs profissional com níveis e contexto
- ✅ **Middleware de Autenticação**: Proteção dupla com API Token (global) e Session Key (tenant)
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

**🔒 Importante - Isolamento Multi Sessões:**

- `API_TOKEN`: Token global compartilhado (autentica o sistema)
- `SESSION_KEY`: Identificador único do tenant/cliente (isola dados)
- Cada `SESSION_KEY` diferente cria um tenant separado
- Tenants não conseguem ver ou acessar sessões de outros tenants

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
- `SESSION_KEY` - Identificador do tenant/cliente (obrigatório)
- `DB_DRIVER` - Driver do banco (sqlite3 ou postgres)
- `DB_DSN` - String de conexão do banco

**💡 Dica:** Gere tokens seguros em: https://www.strongdm.com/tools/api-key-generator

**🔒 Segurança Multi Sessões:**

- O `SESSION_KEY` identifica e isola cada tenant/cliente
- Cada cliente deve ter seu próprio `SESSION_KEY` único
- Um tenant **nunca** vê sessões de outros tenants

3. Execute com Docker Compose:

```bash
docker-compose up -d
```

4. Veja os logs:

```bash
docker-compose logs -f
```

## 📱 Gerenciamento de Sessões Multi Sessões

Este sistema permite gerenciar múltiplas sessões WhatsApp simultaneamente com **isolamento completo por tenant**.

### 🔒 Como Funciona o Isolamento

- **API_TOKEN**: Autentica o acesso ao sistema (compartilhado)
- **SESSION_KEY**: Identifica o tenant/cliente (único por cliente)
- Cada `SESSION_KEY` funciona como um "namespace" isolado
- Um tenant **só vê e gerencia suas próprias sessões**

### Exemplo de Isolamento

```bash
# Cliente A (SESSION_KEY: cliente-a-123)
curl http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: TOKEN_GLOBAL" \
  -H "SESSIONKEY: cliente-a-123"
# Retorna: Apenas sessões do Cliente A

# Cliente B (SESSION_KEY: cliente-b-456)
curl http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: TOKEN_GLOBAL" \
  -H "SESSIONKEY: cliente-b-456"
# Retorna: Apenas sessões do Cliente B
```

Cada sessão WhatsApp representa uma conta WhatsApp conectada dentro do namespace do tenant.

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

**Listar suas sessões (apenas do seu tenant):**

```bash
curl -X GET http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: seu-api-token" \
  -H "SESSIONKEY: sua-session-key"
```

**⚠️ Nota de Segurança:** Este endpoint retorna **apenas as sessões associadas ao seu SESSION_KEY**. Você não verá sessões de outros tenants/clientes.

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

### 🔒 Autenticação e Isolamento

**Todas as requisições requerem dois headers obrigatórios:**

```
apitoken: seu-api-token         # Token global de autenticação
SESSIONKEY: sua-session-key     # Identificador do tenant (isola dados)
Content-Type: application/json
```

**Importante:**

- ⚠️ O `apitoken` autentica o acesso ao sistema
- 🔒 O `SESSIONKEY` determina qual tenant você é e quais dados você pode acessar
- ✅ Requisições sem `SESSIONKEY` serão rejeitadas com **401 Unauthorized**
- 🚫 Você **nunca** verá dados de outros tenants, independente do endpoint

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

#### 3. Listar Sessões do Tenant

```http
GET /api/v1/whatsapp/sessions
```

**🔒 Isolamento de Segurança:**

- Retorna **apenas** sessões associadas ao `SESSIONKEY` do header
- Impossível ver sessões de outros tenants
- Filtro aplicado automaticamente no backend

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

**Nota:** O campo `tenant_id` não é exposto na API por segurança

````

#### 4. Desconectar Sessão

```http
POST /api/v1/whatsapp/disconnect/{sessionKey}
````

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
  "service": "WhatsApp Bot API (Multi Sessões)",
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
│   │   ├── multitenant_handler.go     # Handlers Multi Sessões
│   │   └── session_handler.go         # Handlers de gerenciamento de sessões
│   ├── middleware/
│   │   └── middleware.go              # Middleware (auth, logging, recovery, CORS)
│   ├── models/
│   │   └── models.go                  # Estruturas de dados
│   ├── repository/
│   │   └── session_repository.go      # Camada de acesso aos dados
│   └── services/
│       ├── whatsapp.go                # Serviço WhatsApp (compatibilidade)
│       └── whatsapp_multitenant.go    # Serviço WhatsApp Multi Sessões
├── migrations/
│   ├── 001_create_whatsapp_sessions.sql  # Migração inicial
│   ├── 002_add_device_jid.sql            # Adiciona campo device_jid
│   └── 003_add_tenant_id.sql             # Adiciona isolamento Multi Sessões
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
├── README.md                          # Documentação
└── SECURITY_UPDATE.md                 # Documentação de segurança Multi Sessões
```

## 🔒 Segurança e Isolamento Multi Sessões

### Autenticação em Duas Camadas

1. **API_TOKEN** (Camada Global)
   - Autentica o acesso ao sistema
   - Compartilhado entre todos os tenants/clientes
   - Valida que a requisição é legítima

2. **SESSION_KEY** (Camada de Tenant)
   - Identifica e isola cada tenant/cliente
   - Único para cada tenant
   - Determina quais dados podem ser acessados

### Isolamento de Dados

- ✅ **Isolamento Total por Tenant**: Cada `SESSION_KEY` funciona como namespace isolado
- ✅ **Impossível Cruzar Dados**: Um tenant nunca vê dados de outros tenants
- ✅ **Filtros Automáticos**: Backend aplica filtros por tenant em todas as queries
- ✅ **Validação de Propriedade**: Operações validam que o recurso pertence ao tenant
- ✅ **Logs por Tenant**: Todas as ações são registradas com identificação do tenant

### Recursos de Segurança

- ✅ Autenticação via API Token e Session Key em todos os endpoints
- ✅ Validação de entrada em todas as requisições
- ✅ Sanitização de números de telefone
- ✅ Limitação de tamanho de upload (50MB padrão)
- ✅ CORS configurável via middleware
- ✅ Timeouts configurados para prevenir ataques
- ✅ Logs de tentativas de acesso não autorizado
- ✅ Isolamento de sessões (Multi Sessões)
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
  "service": "WhatsApp Bot API (Multi Sessões)",
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

| Código                  | Descrição                                    | Status HTTP |
| ----------------------- | -------------------------------------------- | ----------- |
| `AUTH_INVALID`          | Token ou session key inválidos               | 401         |
| `SESSION_KEY_REQUIRED`  | Header SESSIONKEY ausente (obrigatório)      | 401         |
| `UNAUTHORIZED`          | Tentativa de acessar recurso de outro tenant | 401         |
| `INVALID_JSON`          | Corpo da requisição malformado               | 400         |
| `VALIDATION_ERROR`      | Dados de entrada inválidos                   | 400         |
| `INVALID_PHONE`         | Formato de número de telefone inválido       | 400         |
| `SESSION_NOT_FOUND`     | Sessão WhatsApp não encontrada neste tenant  | 404         |
| `SESSION_NOT_CONNECTED` | Sessão não está conectada                    | 400         |
| `SEND_FAILED`           | Falha ao enviar mensagem                     | 500         |
| `MEDIA_DOWNLOAD_FAILED` | Falha ao baixar mídia                        | 500         |
| `INTERNAL_ERROR`        | Erro interno do servidor                     | 500         |

### Exemplos de Erros de Segurança

**SESSION_KEY ausente:**

```json
{
  "status": "error",
  "message": "SESSION_KEY é obrigatório",
  "code": "SESSION_KEY_REQUIRED",
  "timestamp": "2026-01-30T10:30:00Z"
}
```

**Tentativa de acessar sessão de outro tenant:**

```json
{
  "status": "error",
  "message": "Sessão não encontrada para este tenant",
  "code": "SESSION_NOT_FOUND",
  "timestamp": "2026-01-30T10:30:00Z"
}
```

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

## 🏢 Casos de Uso Multi Sessões

### Cenário 1: Agência de Marketing com Múltiplos Clientes

```bash
# Cliente A - E-commerce
export API_TOKEN="token-global-agencia"
export SESSION_A="ecommerce-cliente-a"

curl -X POST http://localhost:8080/api/v1/whatsapp/register \
  -H "apitoken: $API_TOKEN" \
  -H "SESSIONKEY: $SESSION_A" \
  -d '{"whatsappSessionKey": "vendas-loja", "nomePessoa": "Vendedor", "emailPessoa": "vendas@clientea.com"}'

# Cliente B - Restaurante
export SESSION_B="restaurante-cliente-b"

curl -X POST http://localhost:8080/api/v1/whatsapp/register \
  -H "apitoken: $API_TOKEN" \
  -H "SESSIONKEY: $SESSION_B" \
  -d '{"whatsappSessionKey": "pedidos-resto", "nomePessoa": "Atendente", "emailPessoa": "pedidos@clienteb.com"}'

# Resultado: Cada cliente vê apenas suas próprias sessões
```

### Cenário 2: Empresa com Múltiplos Departamentos

```bash
# Departamento de Vendas
curl http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: TOKEN_EMPRESA" \
  -H "SESSIONKEY: dept-vendas-2024"
# Retorna: Apenas sessões do departamento de vendas

# Departamento de Suporte
curl http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: TOKEN_EMPRESA" \
  -H "SESSIONKEY: dept-suporte-2024"
# Retorna: Apenas sessões do departamento de suporte
```

### Cenário 3: SaaS com Múltiplos Clientes

Perfeito para plataformas SaaS que oferecem integração WhatsApp:

```javascript
// Backend da sua plataforma SaaS
async function enviarWhatsApp(clienteId, numero, mensagem) {
  const sessionKey = `saas-cliente-${clienteId}`; // Único por cliente

  await fetch("http://whatsapp-api:8080/api/v1/messages/text", {
    headers: {
      apitoken: process.env.API_TOKEN,
      SESSIONKEY: sessionKey, // Isola dados do cliente
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ number, text: mensagem }),
  });
}
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
2026/01/30 10:30:00 [API] [INFO] Iniciando WhatsApp Bot API Multi Sessões...
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

### 🔒 Segurança e Isolamento Multi Sessões

#### O que mudou na versão 2.0?

A partir da versão 2.0, implementamos isolamento completo por tenant. Cada `SESSION_KEY` funciona como um namespace isolado, garantindo que tenants não vejam ou acessem dados de outros tenants.

#### Como funciona o isolamento de dados?

- O `API_TOKEN` autentica o acesso ao sistema (compartilhado)
- O `SESSION_KEY` identifica o tenant e isola seus dados (único por cliente)
- Todas as queries são automaticamente filtradas por `tenant_id`
- É impossível acessar dados de outros tenants, mesmo tentando

#### Posso ter vários clientes usando o mesmo sistema?

Sim! Esse é exatamente o propósito do sistema multi sessões. Cada cliente recebe um `SESSION_KEY` único e só pode ver/gerenciar suas próprias sessões WhatsApp.

#### E se eu esquecer de passar o SESSION_KEY?

A API retornará `401 Unauthorized`. O header `SESSIONKEY` é **obrigatório** em todos os endpoints para garantir o isolamento de dados.

#### Como migrar sessões existentes para o novo sistema?

Execute a migração `003_add_tenant_id.sql` no banco de dados. Sessões antigas receberão `tenant_id = 'default'`. Use `SESSIONKEY: default` para acessá-las. Veja [SECURITY_UPDATE.md](SECURITY_UPDATE.md) para detalhes.

### 📱 WhatsApp e Sessões

#### Como adicionar múltiplas sessões WhatsApp?

Use o endpoint `/api/v1/whatsapp/register` para cada nova sessão com um `session_key` único dentro do seu tenant.

#### A sessão precisa ser reautenticada toda vez?

Não. As sessões são persistidas no banco de dados e reconectam automaticamente.

#### Quantas sessões posso ter por tenant?

Não há limite técnico. Cada tenant pode ter quantas sessões WhatsApp quiser, limitado apenas pelos recursos do servidor.

### 🚀 Produção e Infraestrutura

#### Posso usar em produção?

Sim! Recomendamos usar PostgreSQL e Docker para ambientes de produção.

#### Como limitar o acesso por IP?

Configure um reverse proxy (nginx, traefik) com regras de IP whitelisting.

#### É possível enviar mensagens para grupos?

Sim, use o JID do grupo no campo `number`. Exemplo: `123456789-1234567890@g.us`

#### Como configurar PostgreSQL?

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

## � Migração v1.x → v2.0

Se você está atualizando de uma versão anterior, siga estes passos:

### 1. Backup do Banco de Dados

```bash
# PostgreSQL
pg_dump -h localhost -U usuario whatsapp_bot > backup_v1.sql

# SQLite
cp whatsapp.db whatsapp_v1_backup.db
```

### 2. Executar Migração

```bash
# PostgreSQL
psql -h localhost -U usuario -d whatsapp_bot -f migrations/003_add_tenant_id.sql

# SQLite
sqlite3 whatsapp.db < migrations/003_add_tenant_id.sql
```

### 3. Atualizar Sessões Existentes

Sessões antigas receberão `tenant_id = 'default'`. Para acessá-las:

```bash
curl http://localhost:8080/api/v1/whatsapp/sessions \
  -H "apitoken: SEU_TOKEN" \
  -H "SESSIONKEY: default"
```

### 4. Migrar Sessões para Novos Tenants (Opcional)

Se você quer associar sessões antigas a tenants específicos:

```sql
-- PostgreSQL/SQLite
UPDATE whatsapp_sessions
SET tenant_id = 'novo-tenant-id'
WHERE whatsapp_session_key = 'sessao-especifica';
```

### 5. Atualizar Código do Cliente

Certifique-se de que todas as requisições incluam o header `SESSIONKEY`:

```javascript
// ANTES (v1.x)
fetch("/api/v1/whatsapp/sessions", {
  headers: {
    apitoken: "TOKEN",
  },
});

// DEPOIS (v2.0)
fetch("/api/v1/whatsapp/sessions", {
  headers: {
    apitoken: "TOKEN",
    SESSIONKEY: "seu-tenant-id", // ← NOVO: Obrigatório
  },
});
```

Para mais detalhes, consulte [SECURITY_UPDATE.md](SECURITY_UPDATE.md).

## �📊 Status do Projeto

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
