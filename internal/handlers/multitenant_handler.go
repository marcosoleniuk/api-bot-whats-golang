package handlers

import (
	"boot-whatsapp-golang/internal/config"
	"boot-whatsapp-golang/internal/middleware"
	"boot-whatsapp-golang/internal/models"
	"boot-whatsapp-golang/internal/repository"
	"boot-whatsapp-golang/internal/services"
	"boot-whatsapp-golang/pkg/logger"
	"boot-whatsapp-golang/pkg/validator"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MultiTenantHandler struct {
	whatsappService   *services.MultiTenantWhatsAppService
	webhookService    *services.WebhookService
	realtimeService   *services.RealtimeService
	sessionRepository *repository.SessionRepository
	messageService    *services.MessageService
	config            *config.Config
	logger            *logger.Logger
	startTime         time.Time
}

func NewMultiTenantHandler(whatsappService *services.MultiTenantWhatsAppService, cfg *config.Config, log *logger.Logger) *MultiTenantHandler {
	return &MultiTenantHandler{
		whatsappService: whatsappService,
		config:          cfg,
		logger:          log,
		startTime:       time.Now(),
	}
}

func (h *MultiTenantHandler) SetWebhookService(ws *services.WebhookService) {
	h.webhookService = ws
}

func (h *MultiTenantHandler) SetRealtimeService(rs *services.RealtimeService) {
	h.realtimeService = rs
}

func (h *MultiTenantHandler) SetSessionRepository(sr *repository.SessionRepository) {
	h.sessionRepository = sr
}

func (h *MultiTenantHandler) SetMessageService(ms *services.MessageService) {
	h.messageService = ms
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

	messageID, err := h.whatsappService.SendTextMessage(sessionKey, req.Number, req.Text)
	if err != nil {
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
		MessageID: messageID,
		Recipient: req.Number,
		Type:      "text",
		SentAt:    time.Now(),
	}

	if h.messageService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			_, errSave := h.messageService.SaveOutboundMessage(sessionID, tenantID, messageID, req.Number, req.Text, "text")
			if errSave != nil {
				h.logger.Warnf("[%s] Falha ao salvar mensagem no banco: %v", sessionKey, errSave)
			}
		} else {
			h.logger.Warnf("[%s] Falha ao obter sessionID para salvar mensagem de texto: %v", sessionKey, err)
		}
	}

	if h.webhookService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			webhookData := &models.MessageWebhookData{
				MessageID: messageID,
				To:        req.Number,
				Type:      "text",
				Text:      req.Text,
				Timestamp: messageSent.SentAt,
			}
			go h.webhookService.TriggerWebhooks(sessionID, tenantID, "message.sent", webhookData, sessionKey)
		} else {
			h.logger.Warnf("[%s] Falha ao obter sessionID para webhooks: %v", sessionKey, err)
		}
	}

	h.emitRealtimeEvent(r, sessionKey, "message.sent", &models.MessageWebhookData{
		MessageID: messageID,
		To:        req.Number,
		Type:      "text",
		Text:      req.Text,
		Timestamp: messageSent.SentAt,
	})

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
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

	h.extendWriteDeadlineForLongSend(w, sessionKey, "media")

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

	messageID, detectedMimeType, messageType, mediaBytes, err := h.whatsappService.SendMediaMessage(
		sessionKey,
		req.Number,
		req.Caption,
		req.MediaURL,
		req.MediaBase64,
		req.MimeType,
	)
	if err != nil {
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

	if messageType == "" {
		messageType = "document"
	}
	if req.MimeType == "" {
		req.MimeType = detectedMimeType
	}

	messageSent := models.MessageSent{
		MessageID: messageID,
		Recipient: req.Number,
		Type:      messageType,
		SentAt:    time.Now(),
	}

	if h.messageService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			mediaURL := req.MediaURL
			if mediaURL == "" {
				mediaURL = "[base64-encoded]"
			}
			_, errSave := h.messageService.SaveMediaMessage(
				sessionID,
				tenantID,
				messageID,
				"outbound",
				req.Number,
				messageType,
				mediaURL,
				mediaBytes,
				req.MimeType,
				req.Caption,
			)
			if errSave != nil {
				h.logger.Warnf("[%s] Falha ao salvar mensagem de mídia no banco: %v", sessionKey, errSave)
			}
		} else {
			h.logger.Warnf("[%s] Falha ao obter sessionID para salvar mensagem de mídia: %v", sessionKey, err)
		}
	}

	if h.webhookService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			webhookData := &models.MessageWebhookData{
				MessageID: messageID,
				To:        req.Number,
				Type:      messageType,
				Caption:   req.Caption,
				MediaURL:  req.MediaURL,
				MimeType:  req.MimeType,
				Timestamp: messageSent.SentAt,
			}
			go h.webhookService.TriggerWebhooks(sessionID, tenantID, "message.sent", webhookData, sessionKey)
		} else {
			h.logger.Warnf("[%s] Falha ao obter sessionID para webhooks: %v", sessionKey, err)
		}
	}

	h.emitRealtimeEvent(r, sessionKey, "message.sent", &models.MessageWebhookData{
		MessageID: messageID,
		To:        req.Number,
		Type:      messageType,
		Caption:   req.Caption,
		MediaURL:  req.MediaURL,
		MimeType:  req.MimeType,
		Timestamp: messageSent.SentAt,
	})

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Mensagem de mídia enviada com sucesso",
		messageSent,
	))
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) SendAudioMessage(w http.ResponseWriter, r *http.Request) {
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

	h.extendWriteDeadlineForLongSend(w, sessionKey, "audio")

	var req models.AudioRequest
	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("[%s] JSON inválido na requisição de áudio: %v", sessionKey, err)
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
		h.logger.Warnf("[%s] Número ausente na requisição de áudio", sessionKey)
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
		h.logger.Warnf("[%s] Fonte de mídia ausente na requisição de áudio", sessionKey)
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

	messageID, detectedMimeType, mediaBytes, err := h.whatsappService.SendAudioMessage(
		sessionKey,
		req.Number,
		req.MediaURL,
		req.MediaBase64,
		req.MimeType,
		req.PTT,
	)
	if err != nil {
		h.logger.Errorf("[%s] Falha ao enviar áudio para %s: %v", sessionKey, req.Number, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao enviar áudio",
			"SEND_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}
	if req.MimeType == "" {
		req.MimeType = detectedMimeType
	}

	messageSent := models.MessageSent{
		MessageID: messageID,
		Recipient: req.Number,
		Type:      "audio",
		SentAt:    time.Now(),
	}

	if h.sessionRepository != nil && (h.messageService != nil || h.webhookService != nil) {
		tenantID := middleware.GetTenantID(r)
		mediaURL := req.MediaURL
		if mediaURL == "" {
			mediaURL = "[base64-encoded]"
		}

		mediaBytesToStore := mediaBytes

		go func(
			sessionKey, tenantID, messageID, number, mediaURL, mimeType string,
			mediaBytesToStore []byte,
			sentAt time.Time,
		) {
			sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
			if err != nil {
				h.logger.Warnf("[%s] Falha ao obter sessionID para pós-processamento de áudio: %v", sessionKey, err)
				return
			}

			if h.messageService != nil {
				_, errSave := h.messageService.SaveMediaMessage(
					sessionID,
					tenantID,
					messageID,
					"outbound",
					number,
					"audio",
					mediaURL,
					mediaBytesToStore,
					mimeType,
					"",
				)
				if errSave != nil {
					h.logger.Warnf("[%s] Falha ao salvar áudio no banco: %v", sessionKey, errSave)
				}
			}

			if h.webhookService != nil {
				webhookData := &models.MessageWebhookData{
					MessageID: messageID,
					To:        number,
					Type:      "audio",
					MimeType:  mimeType,
					Timestamp: sentAt,
				}
				h.webhookService.TriggerWebhooks(sessionID, tenantID, "message.sent", webhookData, sessionKey)
			}
		}(sessionKey, tenantID, messageID, req.Number, mediaURL, req.MimeType, mediaBytesToStore, messageSent.SentAt)
	}

	h.emitRealtimeEvent(r, sessionKey, "message.sent", &models.MessageWebhookData{
		MessageID: messageID,
		To:        req.Number,
		Type:      "audio",
		MimeType:  req.MimeType,
		Timestamp: messageSent.SentAt,
	})

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Áudio enviado com sucesso",
		messageSent,
	))
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) SendVideoMessage(w http.ResponseWriter, r *http.Request) {
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

	h.extendWriteDeadlineForLongSend(w, sessionKey, "video")

	var req models.VideoRequest
	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("[%s] JSON inválido na requisição de vídeo: %v", sessionKey, err)
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
		h.logger.Warnf("[%s] Número ausente na requisição de vídeo", sessionKey)
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
		h.logger.Warnf("[%s] Fonte de mídia ausente na requisição de vídeo", sessionKey)
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

	messageID, detectedMimeType, mediaBytes, err := h.whatsappService.SendVideoMessage(
		sessionKey,
		req.Number,
		req.Caption,
		req.MediaURL,
		req.MediaBase64,
		req.MimeType,
		req.GIFPlayback,
	)
	if err != nil {
		h.logger.Errorf("[%s] Falha ao enviar vídeo para %s: %v", sessionKey, req.Number, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao enviar vídeo",
			"SEND_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}
	if req.MimeType == "" {
		req.MimeType = detectedMimeType
	}

	messageSent := models.MessageSent{
		MessageID: messageID,
		Recipient: req.Number,
		Type:      "video",
		SentAt:    time.Now(),
	}

	if h.messageService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			mediaURL := req.MediaURL
			if mediaURL == "" {
				mediaURL = "[base64-encoded]"
			}
			_, errSave := h.messageService.SaveMediaMessage(
				sessionID,
				tenantID,
				messageID,
				"outbound",
				req.Number,
				"video",
				mediaURL,
				mediaBytes,
				req.MimeType,
				req.Caption,
			)
			if errSave != nil {
				h.logger.Warnf("[%s] Falha ao salvar vídeo no banco: %v", sessionKey, errSave)
			}
		} else {
			h.logger.Warnf("[%s] Falha ao obter sessionID para salvar vídeo no banco: %v", sessionKey, err)
		}
	}

	if h.webhookService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			webhookData := &models.MessageWebhookData{
				MessageID: messageID,
				To:        req.Number,
				Type:      "video",
				Caption:   req.Caption,
				MimeType:  req.MimeType,
				Timestamp: messageSent.SentAt,
			}
			go h.webhookService.TriggerWebhooks(sessionID, tenantID, "message.sent", webhookData, sessionKey)
		}
	}

	h.emitRealtimeEvent(r, sessionKey, "message.sent", &models.MessageWebhookData{
		MessageID: messageID,
		To:        req.Number,
		Type:      "video",
		Caption:   req.Caption,
		MimeType:  req.MimeType,
		Timestamp: messageSent.SentAt,
	})

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Vídeo enviado com sucesso",
		messageSent,
	))
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) SendPollMessage(w http.ResponseWriter, r *http.Request) {
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

	var req models.PollRequest
	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("[%s] JSON inválido na requisição de enquete: %v", sessionKey, err)
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
		h.logger.Warnf("[%s] Número ausente na requisição de enquete", sessionKey)
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
	if req.Question == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Campo obrigatório ausente: question",
			"VALIDATION_ERROR",
			map[string]string{"question": "obrigatório"},
		))
		if err != nil {
			return
		}
		return
	}

	options := make([]string, 0, len(req.Options))
	for _, opt := range req.Options {
		trimmed := strings.TrimSpace(opt)
		if trimmed != "" {
			options = append(options, trimmed)
		}
	}
	if len(options) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"A enquete precisa de pelo menos 2 opções válidas",
			"VALIDATION_ERROR",
			map[string]string{"options": "mínimo 2"},
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

	selectableOptionsCount := req.SelectableOptionsCount
	if selectableOptionsCount <= 0 {
		selectableOptionsCount = 1
	}
	if selectableOptionsCount > len(options) {
		selectableOptionsCount = len(options)
	}

	messageID, err := h.whatsappService.SendPollMessage(sessionKey, req.Number, req.Question, options, selectableOptionsCount)
	if err != nil {
		h.logger.Errorf("[%s] Falha ao enviar enquete para %s: %v", sessionKey, req.Number, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao enviar enquete",
			"SEND_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	messageSent := models.MessageSent{
		MessageID: messageID,
		Recipient: req.Number,
		Type:      "poll",
		SentAt:    time.Now(),
	}

	if h.messageService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			_, errSave := h.messageService.SaveOutboundMessage(sessionID, tenantID, messageID, req.Number, req.Question, "unknown")
			if errSave != nil {
				h.logger.Warnf("[%s] Falha ao salvar enquete no banco: %v", sessionKey, errSave)
			}
		} else {
			h.logger.Warnf("[%s] Falha ao obter sessionID para salvar enquete no banco: %v", sessionKey, err)
		}
	}

	if h.webhookService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			webhookData := &models.MessageWebhookData{
				MessageID: messageID,
				To:        req.Number,
				Type:      "poll",
				Text:      req.Question,
				Timestamp: messageSent.SentAt,
			}
			go h.webhookService.TriggerWebhooks(sessionID, tenantID, "message.sent", webhookData, sessionKey)
		}
	}

	h.emitRealtimeEvent(r, sessionKey, "message.sent", &models.MessageWebhookData{
		MessageID: messageID,
		To:        req.Number,
		Type:      "poll",
		Text:      req.Question,
		Timestamp: messageSent.SentAt,
	})

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Enquete enviada com sucesso",
		messageSent,
	))
	if err != nil {
		return
	}
}

func (h *MultiTenantHandler) SendEventMessage(w http.ResponseWriter, r *http.Request) {
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

	h.extendWriteDeadlineForLongSend(w, sessionKey, "event")

	var req models.EventRequest
	if err := validator.ValidateJSON(r, &req); err != nil {
		h.logger.Warnf("[%s] JSON inválido na requisição de evento: %v", sessionKey, err)
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
	if strings.TrimSpace(req.Name) == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Campo obrigatório ausente: name",
			"VALIDATION_ERROR",
			map[string]string{"name": "obrigatório"},
		))
		if err != nil {
			return
		}
		return
	}
	startTimeRaw := strings.TrimSpace(req.StartTime)
	if startTimeRaw == "" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Campo obrigatório inválido: start_time",
			"VALIDATION_ERROR",
			map[string]string{"start_time": "deve ser data/hora (ex: 2026-02-20T09:00:00)"},
		))
		if err != nil {
			return
		}
		return
	}

	startTime, err := parseDateTimeToUnix(startTimeRaw)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errEncode := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Formato inválido para start_time",
			"VALIDATION_ERROR",
			map[string]string{"start_time": "use ISO-8601, ex: 2026-02-20T09:00:00"},
		))
		if errEncode != nil {
			return
		}
		return
	}

	var endTime int64
	if strings.TrimSpace(req.EndTime) != "" {
		endTime, err = parseDateTimeToUnix(req.EndTime)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			errEncode := json.NewEncoder(w).Encode(models.NewErrorResponse(
				"Formato inválido para end_time",
				"VALIDATION_ERROR",
				map[string]string{"end_time": "use ISO-8601, ex: 2026-02-20T10:00:00"},
			))
			if errEncode != nil {
				return
			}
			return
		}
	}

	nowUnix := time.Now().Unix()
	if startTime <= nowUnix {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"start_time precisa estar no futuro",
			"VALIDATION_ERROR",
			map[string]string{
				"start_time": fmt.Sprintf("valor recebido (%s) já passou", req.StartTime),
			},
		))
		if err != nil {
			return
		}
		return
	}
	if endTime > 0 && endTime < startTime {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"end_time não pode ser menor que start_time",
			"VALIDATION_ERROR",
			map[string]string{"end_time": "inválido"},
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

	if req.HasReminder && req.ReminderOffsetSec <= 0 {
		req.ReminderOffsetSec = 900
	}

	callType := strings.ToLower(strings.TrimSpace(req.CallType))
	if callType != "" && callType != "voice" && callType != "video" {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Formato inválido para call_type",
			"VALIDATION_ERROR",
			map[string]string{"call_type": "use 'voice' ou 'video'"},
		))
		if err != nil {
			return
		}
		return
	}

	forceStructured := true
	if req.ForceStructured != nil {
		forceStructured = *req.ForceStructured
	}

	messageID, err := h.whatsappService.SendEventMessage(
		sessionKey,
		req.Number,
		strings.TrimSpace(req.Name),
		req.Description,
		req.JoinLink,
		startTime,
		endTime,
		req.ExtraGuestsAllowed,
		req.IsScheduleCall,
		req.HasReminder,
		req.ReminderOffsetSec,
		callType,
		forceStructured,
	)
	if err != nil {
		h.logger.Errorf("[%s] Falha ao enviar evento para %s: %v", sessionKey, req.Number, err)
		w.WriteHeader(http.StatusInternalServerError)
		err := json.NewEncoder(w).Encode(models.NewErrorResponse(
			"Falha ao enviar evento",
			"SEND_FAILED",
			map[string]string{"error": err.Error()},
		))
		if err != nil {
			return
		}
		return
	}

	messageSent := models.MessageSent{
		MessageID: messageID,
		Recipient: req.Number,
		Type:      "event",
		SentAt:    time.Now(),
	}

	if h.messageService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			_, errSave := h.messageService.SaveOutboundMessage(sessionID, tenantID, messageID, req.Number, strings.TrimSpace(req.Name), "unknown")
			if errSave != nil {
				h.logger.Warnf("[%s] Falha ao salvar evento no banco: %v", sessionKey, errSave)
			}
		} else {
			h.logger.Warnf("[%s] Falha ao obter sessionID para salvar evento no banco: %v", sessionKey, err)
		}
	}

	if h.webhookService != nil && h.sessionRepository != nil {
		sessionID, err := h.sessionRepository.GetIDBySessionKey(sessionKey)
		if err == nil {
			tenantID := middleware.GetTenantID(r)
			webhookData := &models.MessageWebhookData{
				MessageID: messageID,
				To:        req.Number,
				Type:      "event",
				Text:      strings.TrimSpace(req.Name),
				Timestamp: messageSent.SentAt,
			}
			go h.webhookService.TriggerWebhooks(sessionID, tenantID, "message.sent", webhookData, sessionKey)
		}
	}

	h.emitRealtimeEvent(r, sessionKey, "message.sent", &models.MessageWebhookData{
		MessageID: messageID,
		To:        req.Number,
		Type:      "event",
		Text:      strings.TrimSpace(req.Name),
		Timestamp: messageSent.SentAt,
	})

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(models.NewSuccessResponse(
		"Evento enviado com sucesso",
		messageSent,
	))
	if err != nil {
		return
	}
}

func parseDateTimeToUnix(input string) (int64, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, fmt.Errorf("valor vazio")
	}

	if allDigits(value) {
		ts, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, err
		}
		if ts > 9_999_999_999 {
			ts = ts / 1000
		}
		return ts, nil
	}

	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Unix(), nil
	}

	noTZLayouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
	}
	for _, layout := range noTZLayouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.Unix(), nil
		}
	}

	return 0, fmt.Errorf("formato de data inválido: %s", value)
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func (h *MultiTenantHandler) emitRealtimeEvent(r *http.Request, sessionKey, event string, data interface{}) {
	if h.realtimeService == nil {
		return
	}

	requestTenant := strings.TrimSpace(middleware.GetTenantID(r))
	resolvedTenant := requestTenant
	scopes := []string{sessionKey, requestTenant}

	if h.sessionRepository != nil && strings.TrimSpace(sessionKey) != "" {
		session, err := h.sessionRepository.GetBySessionKey(sessionKey)
		if err != nil {
			h.logger.Warnf("[%s] Falha ao resolver escopos realtime: %v", sessionKey, err)
		} else if session != nil {
			resolvedTenant = firstNonEmpty(strings.TrimSpace(session.TenantID), resolvedTenant)
			scopes = append(scopes, strings.TrimSpace(session.TenantID), strings.TrimSpace(session.WhatsAppSessionKey))
		}
	}

	h.realtimeService.Publish(event, strings.TrimSpace(sessionKey), resolvedTenant, data, uniqueNonEmpty(scopes...)...)
}

func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v != "" {
			return v
		}
	}
	return ""
}

func (h *MultiTenantHandler) extendWriteDeadlineForLongSend(w http.ResponseWriter, sessionKey, endpoint string) {
	rc := http.NewResponseController(w)
	if rc == nil {
		return
	}

	if err := rc.SetWriteDeadline(time.Now().Add(10 * time.Minute)); err != nil {
		h.logger.Warnf("[%s] Não foi possível estender write deadline do endpoint %s: %v", sessionKey, endpoint, err)
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
		Service:   "WhatsApp Bot API (Multi Sessões)",
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
