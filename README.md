# WhatsApp Bot API MOleniuk

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

Uma API profissional para envio de mensagens WhatsApp construída em Go, com arquitetura em camadas, suporte a Docker e configuração via variáveis de ambiente.

## 🚀 Características

- ✅ **Arquitetura Profissional**: Estrutura em camadas (handlers, services, middleware)
- ✅ **Configuração via Ambiente**: Todas as configurações através de variáveis de ambiente
- ✅ **Logging Estruturado**: Sistema de logs profissional com níveis
- ✅ **Middleware de Autenticação**: Proteção com API Token e Session Key
- ✅ **Validação de Dados**: Validação robusta de entrada
- ✅ **Health Check**: Endpoint de monitoramento
- ✅ **Graceful Shutdown**: Desligamento elegante do servidor
- ✅ **Docker Ready**: Dockerfile multi-stage otimizado
- ✅ **Suporte a Mídia**: Envio de imagens, vídeos, áudios e documentos
- ✅ **Compatibilidade**: Endpoints retrocompatíveis

## 📋 Pré-requisitos

- Go 1.25 ou superior
- SQLite3
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
API_TOKEN=seu-token-secreto-aqui
SESSION_KEY=sua-chave-de-sessao-aqui
```

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

Você pode gerar tokens seguros em: https://www.strongdm.com/tools/api-key-generator

3. Execute com Docker Compose:

```bash
docker-compose up -d
```

4. Veja os logs:

```bash
docker-compose logs -f
```

## 📱 Primeira Conexão

Na primeira execução, você precisará escanear um QR Code para autenticar o WhatsApp:

1. Execute a aplicação
2. Um QR Code será exibido no terminal
3. Abra o WhatsApp no seu celular
4. Vá em **Configurações** > **Aparelhos Conectados** > **Conectar um Aparelho**
5. Escaneie o QR Code exibido no terminal

A sessão será salva e você não precisará escanear novamente nas próximas execuções.

## 🔌 Endpoints da API

### Health Check

```http
GET /health
```

**Resposta:**

```json
{
  "status": "healthy",
  "service": "WhatsApp Bot API",
  "version": "1.0.0",
  "uptime": "2h30m15s",
  "timestamp": "2026-01-29T10:30:00Z",
  "checks": {
    "whatsapp": "connected",
    "database": "ok"
  }
}
```

### Enviar Mensagem de Texto

```http
POST /api/v1/messages/text
```

**Headers:**

```
apitoken: seu-api-token
SESSIONKEY: sua-session-key
Content-Type: application/json
```

**Body:**

```json
{
  "number": "5511999999999",
  "text": "Olá! Esta é uma mensagem de teste."
}
```

**Resposta:**

```json
{
  "status": "success",
  "message": "Message sent successfully",
  "data": {
    "recipient": "5511999999999",
    "type": "text",
    "sent_at": "2026-01-29T10:30:00Z"
  },
  "timestamp": "2026-01-29T10:30:00Z"
}
```

### Enviar Mensagem de Mídia

```http
POST /api/v1/messages/media
```

**Headers:**

```
apitoken: seu-api-token
SESSIONKEY: sua-session-key
Content-Type: application/json
```

**Body (com URL):**

```json
{
  "number": "5511999999999",
  "caption": "Confira esta imagem!",
  "media_url": "https://example.com/image.jpg"
}
```

**Body (com Base64):**

```json
{
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
  "message": "Media message sent successfully",
  "data": {
    "recipient": "5511999999999",
    "type": "media",
    "sent_at": "2026-01-29T10:30:00Z"
  },
  "timestamp": "2026-01-29T10:30:00Z"
}
```

### Endpoints Retrocompatíveis

Os seguintes endpoints ainda funcionam para compatibilidade:

```http
POST /sendText
POST /sendMedia
```

## ⚙️ Configuração

Todas as configurações são feitas através de variáveis de ambiente:

| Variável                   | Descrição                        | Padrão                              |
| -------------------------- | -------------------------------- | ----------------------------------- |
| `SERVER_PORT`              | Porta do servidor HTTP           | `8080`                              |
| `SERVER_READ_TIMEOUT`      | Timeout de leitura               | `15s`                               |
| `SERVER_WRITE_TIMEOUT`     | Timeout de escrita               | `15s`                               |
| `SERVER_IDLE_TIMEOUT`      | Timeout de idle                  | `60s`                               |
| `SERVER_SHUTDOWN_TIMEOUT`  | Timeout de shutdown              | `10s`                               |
| `MAX_UPLOAD_SIZE`          | Tamanho máximo de upload (bytes) | `52428800` (50MB)                   |
| `WHATSAPP_SESSION_KEY`     | Chave da sessão WhatsApp         | `default-session`                   |
| `WHATSAPP_DEFAULT_COUNTRY` | Código do país padrão            | `55`                                |
| `WHATSAPP_QR_GENERATE`     | Gerar QR Code no terminal        | `true`                              |
| `API_TOKEN`                | Token de autenticação da API     | **OBRIGATÓRIO**                     |
| `SESSION_KEY`              | Chave de sessão                  | **OBRIGATÓRIO**                     |
| `DB_DRIVER`                | Driver do banco de dados         | `sqlite3`                           |
| `DB_DSN`                   | DSN do banco de dados            | `file:whatsapp.db?_foreign_keys=on` |

## 🏗️ Estrutura do Projeto

```
boot-whatsapp-golang/
├── cmd/
│   └── api/
│       └── main.go              # Ponto de entrada da aplicação
├── internal/
│   ├── config/
│   │   └── config.go            # Configuração centralizada
│   ├── handlers/
│   │   └── handlers.go          # HTTP handlers
│   ├── middleware/
│   │   └── middleware.go        # Middleware (auth, logging, recovery)
│   ├── models/
│   │   └── models.go            # Estruturas de dados
│   └── services/
│       └── whatsapp.go          # Lógica de negócio WhatsApp
├── pkg/
│   ├── logger/
│   │   └── logger.go            # Sistema de logging
│   └── validator/
│       └── validator.go         # Validações
├── .env.example                 # Exemplo de configuração
├── .gitignore                   # Arquivos ignorados pelo Git
├── docker-compose.yml           # Configuração Docker Compose
├── Dockerfile                   # Dockerfile multi-stage
├── go.mod                       # Dependências Go
├── go.sum                       # Checksums das dependências
└── README.md                    # Documentação
```

## 🔒 Segurança

- ✅ Autenticação via API Token e Session Key
- ✅ Validação de entrada em todas as requisições
- ✅ Limitação de tamanho de upload
- ✅ CORS configurável
- ✅ Timeouts configurados
- ✅ Logs de tentativas de acesso não autorizado

## 📊 Monitoramento

A API possui um endpoint de health check que pode ser usado para monitoramento:

```bash
curl http://localhost:8080/health
```

Este endpoint verifica:

- Status da conexão WhatsApp
- Status do banco de dados
- Tempo de uptime
- Versão da API

## 🐛 Tratamento de Erros

Todos os erros seguem um formato padronizado:

```json
{
  "status": "error",
  "message": "Descrição do erro",
  "code": "ERROR_CODE",
  "details": {
    "field": "informação adicional"
  },
  "timestamp": "2026-01-29T10:30:00Z"
}
```

Códigos de erro comuns:

- `AUTH_INVALID`: Credenciais inválidas
- `INVALID_JSON`: JSON malformado
- `VALIDATION_ERROR`: Erro de validação de dados
- `INVALID_PHONE`: Número de telefone inválido
- `SEND_FAILED`: Falha ao enviar mensagem
- `INTERNAL_ERROR`: Erro interno do servidor

## 🧪 Testando a API

### Com cURL

```bash
# Health Check
curl http://localhost:8080/health

# Enviar mensagem de texto
curl -X POST http://localhost:8080/api/v1/messages/text \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: sua-chave" \
  -H "Content-Type: application/json" \
  -d '{
    "number": "5511999999999",
    "text": "Teste de mensagem"
  }'

# Enviar imagem
curl -X POST http://localhost:8080/api/v1/messages/media \
  -H "apitoken: seu-token" \
  -H "SESSIONKEY: sua-chave" \
  -H "Content-Type: application/json" \
  -d '{
    "number": "5511999999999",
    "caption": "Imagem de teste",
    "media_url": "https://picsum.photos/200"
  }'
```

### Com Postman

1. Importe a coleção de exemplos (veja pasta `docs/`)
2. Configure as variáveis de ambiente
3. Execute as requisições

## 🔄 Atualizando

Para atualizar a aplicação:

```bash
# Parar a aplicação
docker-compose down

# Atualizar código
git pull

# Reconstruir e iniciar
docker-compose up -d --build
```

## 📝 Logs

Os logs são estruturados e incluem:

- Timestamp
- Nível (DEBUG, INFO, WARN, ERROR, FATAL)
- Módulo
- Mensagem

Exemplo:

```
2026/01/29 10:30:00 [API] [INFO] Configuration loaded successfully
2026/01/29 10:30:01 [WhatsApp] [INFO] Successfully connected to WhatsApp
2026/01/29 10:30:02 [API] [INFO] API server listening on port 8080
```

## 🤝 Contribuindo

Contribuições são bem-vindas! Por favor:

1. Faça um fork do projeto
2. Crie uma branch para sua feature (`git checkout -b feature/MinhaFeature`)
3. Commit suas mudanças (`git commit -m 'Adiciona MinhaFeature'`)
4. Push para a branch (`git push origin feature/MinhaFeature`)
5. Abra um Pull Request

## 📄 Licença

Este projeto está sob a licença MIT. Veja o arquivo `LICENSE` para mais detalhes.

## 🆘 Suporte

Se você encontrar algum problema ou tiver dúvidas:

1. Verifique os logs: `docker-compose logs -f`
2. Consulte a seção de troubleshooting
3. Abra uma issue no GitHub

## 📚 Recursos Adicionais

- [Documentação WhatsApp Business API](https://developers.facebook.com/docs/whatsapp)
- [Whatsmeow Library](https://github.com/tulir/whatsmeow)
- [Go Documentation](https://golang.org/doc/)

---

**Desenvolvido com usando Go**
