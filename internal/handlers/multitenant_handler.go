package handlers

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"boot-whatsapp-golang/pkg/validator"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type MultiTenantHandler struct {
	whatsappService *services.MultiTenantWhatsAppService
	config          *config.Config
	logger          *logger.Logger
	startTime       time.Time
}

func NewMultiTenantHandler(whatsappService *services.MultiTenantWhatsAppService, cfg *config.Config, log *logger.Logger) *MultiTenantHandler {
	return &MultiTenantHandler{
		whatsappService: whatsappService,
		config:          cfg,
		logger:          log,
		startTime:       time.Now(),
	}
}

func (h *MultiTenantHandler) SendTextMessage(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.Header.Get("X-WhatsApp-Session-Key")
	if sessionKey == "" {
		h.logger.Warn("whatsappSessionKey ausente no header")
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Header X-WhatsApp-Session-Key é obrigatório",
			"MISSING_SESSION_KEY",
			nil,
		))
		if err != nil {
			return
		}
		return
	}

	var req models.MessageRequest

	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("[%s] JSON inválido na requisição de mensagem de texto: %v", sessionKey, err)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Corpo da requisição inválido",
			"INVALID_JSON",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	if req.Number == "" || req.Text == "" {
		h.logger.Warnf("[%s] Campos obrigatórios ausentes na requisição de mensagem de texto", sessionKey)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Campos obrigatórios ausentes",
			"VALIDATION_ERROR",
			map[string]string{
				"number": "obrigatório",
				"text":   "obrigatório",
			},
		))
		if err != nil {
			return
		}
		return
	}

	if err := validator.ValidatePhoneNumber(req.Number); err != nil {
		h.logger.Warnf("[%s] Número de telefone inválido: %v", sessionKey, err)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Formato de número de telefone inválido",
			"INVALID_PHONE",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	if err := h.whatsappService.SendTextMessage(sessionKey, req.Number, req.Text); err != nil {
		h.logger.Errorf("[%s] Falha ao enviar mensagem de texto para %s: %v", sessionKey, req.Number, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao enviar mensagem",
			"SEND_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	messageSent := models.MessageSent{
		Recipient: req.Number,
		Type:      "text",
		SentAt:    time.Now(),
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Mensagem enviada com sucesso",
		messageSent,
	))
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) SendMediaMessage(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.Header.Get("X-WhatsApp-Session-Key")
	if sessionKey == "" {
		h.logger.Warn("whatsappSessionKey ausente no header")
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Header X-WhatsApp-Session-Key é obrigatório",
			"MISSING_SESSION_KEY",
			nil,
		))
		if err != nil {
			return
		}
		return
	}

	var req models.MediaRequest

	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("[%s] JSON inválido na requisição de mensagem de mídia: %v", sessionKey, err)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Corpo da requisição inválido",
			"INVALID_JSON",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	if req.Number == "" {
		h.logger.Warnf("[%s] Número ausente na requisição de mensagem de mídia", sessionKey)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Campo obrigatório ausente: número",
			"VALIDATION_ERROR",
			map[string]string{"number": "obrigatório"},
		))
		if err != nil {
			return
		}
		return
	}

	if req.MediaURL == "" && req.MediaBase64 == "" {
		h.logger.Warnf("[%s] Fonte de mídia ausente na requisição de mensagem de mídia", sessionKey)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"É necessário fornecer media_url ou media_base64",
			"VALIDATION_ERROR",
			map[string]string{
				"media_url":    "obrigatório_sem media_base64",
				"media_base64": "obrigatório_sem media_url",
			},
		))
		if err != nil {
			return
		}
		return
	}

	if req.MediaBase64 != "" && req.MimeType == "" {
		h.logger.Warnf("[%s] mime_type ausente para mídia base64", sessionKey)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"mime_type é obrigatório ao usar media_base64",
			"VALIDATION_ERROR",
			map[string]string{"mime_type": "obrigatório com media_base64"},
		))
		if err != nil {
			return
		}
		return
	}

	if err := validator.ValidatePhoneNumber(req.Number); err != nil {
		h.logger.Warnf("[%s] Número de telefone inválido: %v", sessionKey, err)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Formato de número de telefone inválido",
			"INVALID_PHONE",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	if err := h.whatsappService.SendMediaMessage(sessionKey, req.Number, req.Caption, req.MediaURL, req.MediaBase64, req.MimeType); err != nil {
		h.logger.Errorf("[%s] Falha ao enviar mensagem de mídia para %s: %v", sessionKey, req.Number, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao enviar mensagem de mídia",
			"SEND_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	messageSent := models.MessageSent{
		Recipient: req.Number,
		Type:      "media",
		SentAt:    time.Now(),
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Mensagem de mídia enviada com sucesso",
		messageSent,
	))
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) Health(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)

	sessions, err := h.whatsappService.ListSessions()
	sessionsCount := "0"
	connectedCount := "0"
	if err == nil {
		sessionsCount = fmt.Sprintf("%d", len(sessions))
		connected := 0
		for _, s := range sessions {
			if s.Status == models.SessionStatusConnected {
				connected++
			}
		}
		connectedCount = fmt.Sprintf("%d", connected)
	}

	health := models.HealthResponse{
		Status:    "healthy",
		Service:   "WhatsApp Bot API (Multi-Tenant)",
		Version:   "2.0.0",
		Uptime:    uptime.String(),
		Timestamp: time.Now(),
		Checks: map[string]string{
			"api":                "ok",
			"total_sessions":     sessionsCount,
			"connected_sessions": connectedCount,
		},
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(health)
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	err := json.NewEncoder(w).Encode(models.NewErrorResponse(
		"Endpoint não encontrado",
		"NOT_FOUND",
		map[string]string{"path": r.URL.Path},
	))
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	err := json.NewEncoder(w).Encode(models.NewErrorResponse(
		"Método não permitido",
		"METHOD_NOT_ALLOWED",
		map[string]string{"method": r.Method},
	))
	if err != nil {
		return
	}
}
