package main

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/handlers"
	"boot-whatsapp-golang/internal/middleware"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
)

const (
	Version = "1.0.0"
	Banner  = `
╔══════════════════════════════════════════════════════════╗
║                                                          ║
║        WhatsApp Bot API - MOleniuk                       ║
║                    Version %s                            ║
║                                                          ║
╚══════════════════════════════════════════════════════════╝
`
)

func main() {
	fmt.Printf(Banner, Version)

	log := logger.New("[API] ", logger.INFO)
	log.Info("Iniciando WhatsApp Bot API...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Falha ao carregar configuração: %v", err)
	}
	log.Info("Configuração carregada com sucesso")

	whatsappService, err := services.NewWhatsAppService(cfg, log)
	if err != nil {
		log.Fatalf("Falha ao inicializar serviço WhatsApp: %v", err)
	}
	log.Info("Serviço WhatsApp inicializado")

	if err := whatsappService.Connect(); err != nil {
		log.Fatalf("Falha ao conectar ao WhatsApp: %v", err)
	}
	log.Info("Conectado ao WhatsApp com sucesso")

	handler := handlers.NewHandler(whatsappService, cfg, log)

	mux := setupRouter(handler, cfg, log)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Infof("Servidor API escutando na porta %s", cfg.Server.Port)
		log.Infof("Health check disponível em: http://localhost:%s/health", cfg.Server.Port)
		log.Infof("Endpoints da API disponíveis em: http://localhost:%s/api/v1/*", cfg.Server.Port)

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

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		log.Info("Encerrando servidor...")
		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Erro ao encerrar servidor: %v", err)
			if err := server.Close(); err != nil {
				log.Errorf("Erro ao fechar servidor: %v", err)
			}
		}

		log.Info("Desconectando do WhatsApp...")
		whatsappService.Disconnect()

		log.Info("Servidor encerrado com sucesso")
	}
}

func setupRouter(h *handlers.Handler, cfg *config.Config, log *logger.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/", h.NotFound)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/messages/text", methodFilter(h.SendTextMessage, http.MethodPost, h))
	apiMux.HandleFunc("/api/v1/messages/media", methodFilter(h.SendMediaMessage, http.MethodPost, h))

	apiMux.HandleFunc("/sendText", methodFilter(h.SendTextMessage, http.MethodPost, h))
	apiMux.HandleFunc("/sendMedia", methodFilter(h.SendMediaMessage, http.MethodPost, h))

	handler := applyMiddleware(
		apiMux,
		middleware.RecoveryMiddleware(log),
		middleware.LoggingMiddleware(log),
		middleware.CORSMiddleware(),
		middleware.ContentTypeMiddleware(),
		middleware.AuthMiddleware(cfg, log),
	)

	mux.Handle("/api/", handler)
	mux.Handle("/sendText", handler)
	mux.Handle("/sendMedia", handler)

	return mux
}

func methodFilter(handler http.HandlerFunc, allowedMethod string, h *handlers.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != allowedMethod {
			h.MethodNotAllowed(w, r)
			return
		}
		handler(w, r)
	}
}

func applyMiddleware(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}
