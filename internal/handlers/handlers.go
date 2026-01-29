package handlers

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"boot-whatsapp-golang/pkg/validator"
	"encoding/json"
	"net/http"
	"time"
)

type Handler struct {
	whatsappService *services.WhatsAppService
	config          *config.Config
	logger          *logger.Logger
	startTime       time.Time
}

func NewHandler(whatsappService *services.WhatsAppService, cfg *config.Config, log *logger.Logger) *Handler {
	return &Handler{
		whatsappService: whatsappService,
		config:          cfg,
		logger:          log,
		startTime:       time.Now(),
	}
}

func (h *Handler) SendTextMessage(w http.ResponseWriter, r *http.Request) {
	var req models.MessageRequest

	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("JSON inválido na requisição de mensagem de texto: %v", err)
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
		h.logger.Warn("Campos obrigatórios ausentes na requisição de mensagem de texto")
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
		h.logger.Warnf("Invalid phone number: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Invalid phone number format",
			"INVALID_PHONE",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	if err := h.whatsappService.SendTextMessage(req.Number, req.Text); err != nil {
		h.logger.Errorf("Falha ao enviar mensagem de texto para %s: %v", req.Number, err)
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

func (h *Handler) SendMediaMessage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.config.Server.MaxUploadSize)

	var req models.MediaRequest

	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("JSON inválido na requisição de mensagem de mídia: %v", err)
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
		h.logger.Warn("Número ausente na requisição de mensagem de mídia")
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
		h.logger.Warn("Fonte de mídia ausente na requisição de mensagem de mídia")
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
		h.logger.Warn("mime_type ausente para mídia base64")
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
		h.logger.Warnf("Invalid phone number: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Invalid phone number format",
			"INVALID_PHONE",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	if err := h.whatsappService.SendMediaMessage(req.Number, req.Caption, req.MediaURL, req.MediaBase64, req.MimeType); err != nil {
		h.logger.Errorf("Falha ao enviar mensagem de mídia para %s: %v", req.Number, err)
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

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)

	checks := map[string]string{
		"whatsapp": "disconnected",
		"database": "ok",
	}

	if h.whatsappService.IsConnected() {
		checks["whatsapp"] = "connected"
	}

	status := "healthy"
	if checks["whatsapp"] == "disconnected" {
		status = "degraded"
	}

	response := models.HealthResponse{
		Status:    status,
		Service:   "WhatsApp Bot API",
		Version:   "1.0.0",
		Uptime:    uptime.String(),
		Timestamp: time.Now(),
		Checks:    checks,
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
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
