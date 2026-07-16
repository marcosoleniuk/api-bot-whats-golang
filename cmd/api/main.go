package main

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/handlers"
	"boot-whatsapp-golang/internal/middleware"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

const (
	Version = "2.0.0"
	Banner  = `
╔══════════════════════════════════════════════════════════╗
║                                                          ║
║    WhatsApp Bot API (Multi Sessões) - MOleniuk           ║
║                    Version %s                         ║
║                                                          ║
╚══════════════════════════════════════════════════════════╝
`
)

func main() {
	fmt.Printf(Banner, Version)

	log := logger.New("[API] ", logger.INFO)
	log.Info("Iniciando WhatsApp Bot API Multi Sessões...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Falha ao carregar configuração: %v", err)
	}
	log.Info("Configuração carregada com sucesso")

	db, err := sql.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Falha ao conectar ao banco de dados: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Errorf("Erro ao fechar conexão com banco de dados: %v", err)
		}
	}(db)

	if err := db.Ping(); err != nil {
		log.Fatalf("Falha ao verificar conexão com banco de dados: %v", err)
	}
	log.Info("Conectado ao banco de dados com sucesso")

	whatsappService, err := services.NewMultiTenantWhatsAppService(cfg, db, log)
	if err != nil {
		log.Fatalf("Falha ao inicializar serviço WhatsApp: %v", err)
	}
	log.Info("Serviço WhatsApp Multi Sessões inicializado")

	sessionRepo := repository.NewSessionRepository(db, log)
	messageRepo := repository.NewMessageRepository(db)

	messageHandler := handlers.NewMultiTenantHandler(whatsappService, cfg, log)
	sessionHandler := handlers.NewSessionHandler(whatsappService, log)

	webhookRepo := repository.NewWebhookRepository(db, log)
	webhookService := services.NewWebhookService(webhookRepo, log)
	webhookHandler := handlers.NewWebhookHandler(webhookService, sessionRepo, log)

	messageService := services.NewMessageService(messageRepo, log)
	realtimeService := services.NewRealtimeService(log)
	realtimeHandler := handlers.NewRealtimeHandler(realtimeService, log)
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	messageService.StartMediaCleanupJob(appCtx)
	conversationHandler := handlers.NewConversationHandler(messageService, sessionRepo, log)

	messageHandler.SetWebhookService(webhookService)
	messageHandler.SetRealtimeService(realtimeService)
	messageHandler.SetSessionRepository(sessionRepo)
	messageHandler.SetMessageService(messageService)

	whatsappService.SetWebhookService(webhookService)
	whatsappService.SetMessageService(messageService)
	whatsappService.SetRealtimeService(realtimeService)
	whatsappService.StartConnectionMonitor(appCtx)

	router := setupRouter(messageHandler, sessionHandler, webhookHandler, conversationHandler, realtimeHandler, cfg, log)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Infof("Servidor API escutando na porta %s", cfg.Server.Port)
		log.Infof("Health check disponível em: http://localhost:%s/health", cfg.Server.Port)
		log.Info("Endpoints disponíveis:")
		log.Info("  POST /api/v1/whatsapp/register - Registrar nova sessão WhatsApp")
		log.Info("  GET  /api/v1/whatsapp/qrcode/{sessionKey} - Obter QR code de sessão")
		log.Info("  GET  /api/v1/whatsapp/sessions - Listar todas as sessões")
		log.Info("  POST /api/v1/whatsapp/disconnect/{sessionKey} - Desconectar sessão")
		log.Info("  DELETE /api/v1/whatsapp/sessions/{sessionKey} - Deletar sessão")
		log.Info("  POST /api/v1/messages/text - Enviar mensagem de texto")
		log.Info("  POST /api/v1/messages/media - Enviar mensagem com mídia")
		log.Info("  POST /api/v1/messages/audio - Enviar áudio")
		log.Info("  POST /api/v1/messages/video - Enviar vídeo")
		log.Info("  POST /api/v1/messages/poll - Enviar enquete")
		log.Info("  POST /api/v1/messages/event - Enviar evento")
		log.Info("  GET  /api/v1/ws - WebSocket de eventos em tempo real")
		log.Info("  POST /api/v1/webhooks - Registrar novo webhook")
		log.Info("  GET  /api/v1/webhooks - Listar webhooks")
		log.Info("  GET  /api/v1/webhooks/{webhookId} - Obter webhook")
		log.Info("  PUT  /api/v1/webhooks/{webhookId} - Atualizar webhook")
		log.Info("  DELETE /api/v1/webhooks/{webhookId} - Deletar webhook")
		log.Info("  GET  /api/v1/webhooks/{webhookId}/logs - Obter logs do webhook")

		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Erro no servidor: %v", err)
		}
	case sig := <-shutdown:
		log.Infof("Sinal de desligamento recebido: %v", sig)
		appCancel()

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		log.Info("Encerrando servidor...")
		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Erro ao encerrar servidor: %v", err)
			if err := server.Close(); err != nil {
				log.Errorf("Erro ao fechar servidor: %v", err)
			}
		}

		log.Info("Desconectando todas as sessões WhatsApp...")
		whatsappService.Shutdown()

		log.Info("Servidor encerrado com sucesso")
	}
}

func setupRouter(
	mh *handlers.MultiTenantHandler,
	sh *handlers.SessionHandler,
	wh *handlers.WebhookHandler,
	ch *handlers.ConversationHandler,
	rh *handlers.RealtimeHandler,
	cfg *config.Config,
	log *logger.Logger,
) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/health", mh.Health).Methods("GET")

	api := r.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/whatsapp/register", sh.RegisterSession).Methods("POST")
	api.HandleFunc("/whatsapp/qrcode/{sessionKey}", sh.GetQRCode).Methods("GET")
	api.HandleFunc("/whatsapp/sessions", sh.ListSessions).Methods("GET")
	api.HandleFunc("/whatsapp/disconnect/{sessionKey}", sh.DisconnectSession).Methods("POST")
	api.HandleFunc("/whatsapp/sessions/{sessionKey}", sh.DeleteSession).Methods("DELETE")

	api.HandleFunc("/messages/text", mh.SendTextMessage).Methods("POST")
	api.HandleFunc("/messages/media", mh.SendMediaMessage).Methods("POST")
	api.HandleFunc("/messages/audio", mh.SendAudioMessage).Methods("POST")
	api.HandleFunc("/messages/video", mh.SendVideoMessage).Methods("POST")
	api.HandleFunc("/messages/poll", mh.SendPollMessage).Methods("POST")
	api.HandleFunc("/messages/event", mh.SendEventMessage).Methods("POST")

	api.HandleFunc("/messages/history", ch.GetConversationWithAuth).Methods("GET")
	api.HandleFunc("/messages/contacts", ch.GetContactsWithAuth).Methods("GET")
	api.HandleFunc("/messages/stats", ch.GetStatsWithAuth).Methods("GET")
	api.HandleFunc("/messages/{messageID}/media/{filename:.*}", ch.GetMessageMedia).Methods("GET")
	api.HandleFunc("/ws", rh.Connect).Methods("GET")

	api.HandleFunc("/conversations/{sessionID}", ch.GetConversation).Methods("GET")
	api.HandleFunc("/conversations/{sessionID}/contacts", ch.GetContacts).Methods("GET")
	api.HandleFunc("/conversations/{sessionID}/stats", ch.GetMessageStats).Methods("GET")

	api.HandleFunc("/webhooks", wh.RegisterWebhook).Methods("POST")
	api.HandleFunc("/webhooks", wh.ListWebhooks).Methods("GET")
	api.HandleFunc("/webhooks/{webhookId}", wh.GetWebhook).Methods("GET")
	api.HandleFunc("/webhooks/{webhookId}", wh.UpdateWebhook).Methods("PUT")
	api.HandleFunc("/webhooks/{webhookId}", wh.DeleteWebhook).Methods("DELETE")
	api.HandleFunc("/webhooks/{webhookId}/logs", wh.GetWebhookLogs).Methods("GET")

	api.HandleFunc("/sendText", mh.SendTextMessage).Methods("POST")
	api.HandleFunc("/sendMedia", mh.SendMediaMessage).Methods("POST")
	api.HandleFunc("/sendAudio", mh.SendAudioMessage).Methods("POST")
	api.HandleFunc("/sendVideo", mh.SendVideoMessage).Methods("POST")
	api.HandleFunc("/sendPoll", mh.SendPollMessage).Methods("POST")
	api.HandleFunc("/sendEvent", mh.SendEventMessage).Methods("POST")

	r.Use(middleware.RecoveryMiddleware(log))
	r.Use(middleware.LoggingMiddleware(log))
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ContentTypeMiddleware())

	api.Use(func(next http.Handler) http.Handler {
		return middleware.AuthMiddleware(cfg, log)(next)
	})

	r.NotFoundHandler = http.HandlerFunc(mh.NotFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(mh.MethodNotAllowed)

	return r
}
